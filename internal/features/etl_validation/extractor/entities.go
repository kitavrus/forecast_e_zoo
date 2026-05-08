package extractor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// NDJSONScannerBufSize — максимальный размер одной NDJSON-строки.
const NDJSONScannerBufSize = 1 << 20 // 1 MiB

// dateLayout — формат для query-параметров event_date_from / event_date_to.
const dateLayout = "2006-01-02"

// nextCursorHeader — header, в котором source-adapter возвращает курсор
// следующей страницы (см. internal/features/data_export/handler/streaming.go
// WriteNextCursor). Пустое значение / отсутствие header → конец стрима.
const nextCursorHeader = "X-Next-Cursor"

// pageFetchSafetyCap — мягкий потолок числа страниц на одну Stream() вызов.
// 1.6M строк / 1000 ≈ 1600 страниц; ставим 100k чтобы поймать «бесконечный
// цикл» (например баг сервера, не сбрасывающий cursor).
const pageFetchSafetyCap = 100_000

// FactEntities — entities, для которых source-adapter требует обязательный
// event_date_from / event_date_to (партиционированные таблицы).
//
// См. internal/features/data_export/handler/receipt_line.go — handler возвращает
// 400 Bad Request, если не передать оба параметра.
//
//nolint:gochecknoglobals // справочный список, single-source-of-truth.
var FactEntities = map[string]struct{}{
	"receipt_line":            {},
	"location_stock_snapshot": {},
	"stock_movement":          {},
	"supplier_stock_snapshot": {},
}

// IsFactEntity возвращает true, если entity — fact (партиционированная таблица),
// и для GET /v1/{entity} обязательны event_date_from / event_date_to.
func IsFactEntity(entity string) bool {
	_, ok := FactEntities[entity]
	return ok
}

// EntitiesClient — публичный интерфейс для тестов EtlPipeline.
//
// from / to:
//   - для master-сущностей — передавать time.Time{} (не выставляется since/date).
//   - для facts — обязательны (event_date_from / event_date_to, формат YYYY-MM-DD).
type EntitiesClient interface {
	Stream(ctx context.Context, entity, snapshotID, etag string, from, to time.Time) (NDJSONReader, error)
}

// NDJSONReader — итератор по строкам NDJSON.
type NDJSONReader interface {
	// Next декодирует следующую строку в target. На EOF возвращает io.EOF.
	Next(target any) error
	// ETag возвращает значение заголовка ETag (если был).
	ETag() string
	// Close — освобождает ресурсы (закрывает body).
	Close() error
}

type entitiesClient struct {
	c *Client
}

// NewEntitiesClient оборачивает Client.
func NewEntitiesClient(c *Client) EntitiesClient {
	return &entitiesClient{c: c}
}

// Stream открывает поток GET /v1/{entity}?snapshot_id=...
// и возвращает NDJSONReader. Если etag совпал → 304 Not Modified, NDJSONReader
// окажется пустым (Next сразу вернёт io.EOF) и ETag() вернёт переданный etag.
//
// Для facts-сущностей (см. IsFactEntity) source-adapter требует обязательные
// event_date_from / event_date_to (партиционирование). from/to добавляются в
// query как YYYY-MM-DD, если оба не нулевые. Для master-сущностей — игнорируются.
//
// Pagination: source-adapter отдаёт страницы по 1000 строк (limit по умолчанию)
// и в response header X-Next-Cursor — opaque cursor следующей страницы.
// Stream() возвращает NDJSONReader, который автоматически перелистывает
// все страницы до пустого X-Next-Cursor; для caller это выглядит как один
// непрерывный поток NDJSON. См. internal/features/data_export/handler/streaming.go
// (WriteNextCursor) — handler ставит cursor только при len(rows) == limit.
func (e *entitiesClient) Stream(ctx context.Context, entity, snapshotID, etag string, from, to time.Time) (NDJSONReader, error) {
	if entity == "" {
		return nil, errors.New("extractor: entity is empty")
	}
	if snapshotID == "" {
		return nil, errors.New("extractor: snapshotID is empty")
	}
	pr := &paginatedNDJSONReader{
		ec:         e,
		ctx:        ctx,
		entity:     entity,
		snapshotID: snapshotID,
		ifNoneTag:  etag,
		from:       from,
		to:         to,
		cursor:     "",
		exhausted:  false,
	}
	if err := pr.openNextPage(); err != nil {
		return nil, err
	}
	return pr, nil
}

// fetchPage делает один запрос и возвращает (NDJSONReader-страница, nextCursor, error).
// Используется paginatedNDJSONReader.
//
//nolint:funlen,cyclop // линейный switch по статусам + сборка query.
func (e *entitiesClient) fetchPage(
	ctx context.Context,
	entity, snapshotID, ifNoneTag, cursor string,
	from, to time.Time,
) (page *ndjsonReader, nextCursor string, err error) {
	u, err := url.Parse(e.c.BaseURL() + "/v1/" + url.PathEscape(entity))
	if err != nil {
		return nil, "", fmt.Errorf("extractor: parse url: %w", err)
	}
	q := u.Query()
	q.Set("snapshot_id", snapshotID)
	if !from.IsZero() && !to.IsZero() {
		q.Set("event_date_from", from.UTC().Format(dateLayout))
		q.Set("event_date_to", to.UTC().Format(dateLayout))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("extractor: build entities req: %w", err)
	}
	req.Header.Set("Accept", "application/x-ndjson")
	if ifNoneTag != "" {
		req.Header.Set("If-None-Match", ifNoneTag)
	}

	resp, err := e.c.Do(ctx, req)
	if err != nil {
		return nil, "", err //nolint:wrapcheck // already wrapped
	}

	switch resp.StatusCode {
	case http.StatusOK:
		respETag := resp.Header.Get("ETag")
		return newNDJSONReader(resp.Body, respETag), resp.Header.Get(nextCursorHeader), nil
	case http.StatusNotModified:
		drainAndClose(resp)
		return newNDJSONReader(io.NopCloser(emptyReader{}), ifNoneTag), "", nil
	case http.StatusNotFound, http.StatusNotImplemented:
		// Source-adapter MVP реализует только подмножество entities (см.
		// internal/features/data_export/handler/not_implemented.go: 501 для
		// 14 master/facts entities вне products + receipt_line). Fiber default
		// 404 — для не-зарегистрированных маршрутов.
		//
		// Трактуем оба статуса как «entity ещё не экспортируется» → пустой stream.
		// populateStaging уже толерантен к пустым/неизвестным entities (см. staging.go).
		drainAndClose(resp)
		return newNDJSONReader(io.NopCloser(emptyReader{}), ""), "", nil
	default:
		drainAndClose(resp)
		return nil, "", errorspkg.ErrSourceUnavailable.Wrap(
			fmt.Errorf("unexpected status %d on /v1/%s", resp.StatusCode, entity),
		)
	}
}

// paginatedNDJSONReader — NDJSONReader, который прозрачно склеивает страницы
// X-Next-Cursor pagination в один поток.
//
// Лениво подгружает следующую страницу, когда текущая исчерпана. Не загружает
// все строки в память (≠ http_source_reader.fetchAll, который для упрощения
// аккумулирует master-сущности целиком — там объёмы небольшие).
type paginatedNDJSONReader struct {
	ec         *entitiesClient
	ctx        context.Context //nolint:containedctx // ctx живёт всё время iteration по pageам
	entity     string
	snapshotID string
	ifNoneTag  string // If-None-Match для первой страницы
	from, to   time.Time

	page      *ndjsonReader // текущая открытая страница (nil после Close)
	etag      string        // ETag первой 200/304 страницы (фиксируется только один раз)
	cursor    string        // X-Next-Cursor предыдущей страницы; "" → больше нет
	exhausted bool          // true, если cursor == "" после последнего fetch (больше страниц не будет)
	pages     int           // счётчик страниц (защита от runaway)
}

// openNextPage делает запрос и устанавливает p.page / p.cursor.
// На первом вызове использует p.cursor == "" (initial request).
// Сбрасывает p.exhausted, если сервер вернул следующую страницу.
func (p *paginatedNDJSONReader) openNextPage() error {
	if p.page != nil {
		_ = p.page.Close()
		p.page = nil
	}
	if p.pages >= pageFetchSafetyCap {
		return fmt.Errorf("extractor: pagination safety cap reached (%d pages) on /v1/%s",
			pageFetchSafetyCap, p.entity)
	}
	page, next, err := p.ec.fetchPage(p.ctx, p.entity, p.snapshotID, p.ifNoneTag, p.cursor, p.from, p.to)
	if err != nil {
		return err
	}
	p.pages++
	p.page = page
	// ETag берём только с первой страницы (он привязан к (load_id, entity, committed_at)
	// и не меняется между страницами одного snapshot).
	if p.etag == "" {
		p.etag = page.etag
	}
	// Subsequent requests не должны слать If-None-Match (иначе сервер на page 2+
	// может вернуть 304 для unchanged ETag и оборвать загрузку).
	p.ifNoneTag = ""
	p.cursor = next
	if next == "" {
		p.exhausted = true
	}
	return nil
}

// Next декодирует следующую строку, прозрачно перелистывая страницы.
func (p *paginatedNDJSONReader) Next(target any) error {
	for {
		if p.page == nil {
			return io.EOF
		}
		err := p.page.Next(target)
		if err == nil {
			return nil
		}
		if !errors.Is(err, io.EOF) {
			return err
		}
		// Текущая страница исчерпана. Если есть cursor — открываем следующую.
		if p.exhausted || p.cursor == "" {
			return io.EOF
		}
		if openErr := p.openNextPage(); openErr != nil {
			return openErr
		}
	}
}

// ETag возвращает ETag первой страницы (он стабилен в пределах snapshot).
func (p *paginatedNDJSONReader) ETag() string { return p.etag }

// Close освобождает ресурсы текущей страницы.
func (p *paginatedNDJSONReader) Close() error {
	if p.page == nil {
		return nil
	}
	err := p.page.Close()
	p.page = nil
	return err //nolint:wrapcheck // delegating
}

type ndjsonReader struct {
	body    io.ReadCloser
	scanner *bufio.Scanner
	etag    string
}

func newNDJSONReader(body io.ReadCloser, etag string) *ndjsonReader {
	sc := bufio.NewScanner(body)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, NDJSONScannerBufSize)
	return &ndjsonReader{body: body, scanner: sc, etag: etag}
}

// Next декодирует следующую строку.
func (r *ndjsonReader) Next(target any) error {
	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := json.Unmarshal(line, target); err != nil {
			return fmt.Errorf("extractor: ndjson decode: %w", err)
		}
		return nil
	}
	if err := r.scanner.Err(); err != nil {
		return fmt.Errorf("extractor: ndjson scan: %w", err)
	}
	return io.EOF
}

// ETag возвращает заголовок ответа.
func (r *ndjsonReader) ETag() string { return r.etag }

// Close освобождает body.
func (r *ndjsonReader) Close() error {
	if r.body == nil {
		return nil
	}
	return r.body.Close() //nolint:wrapcheck // delegating
}

type emptyReader struct{}

func (emptyReader) Read(_ []byte) (int, error) { return 0, io.EOF }
