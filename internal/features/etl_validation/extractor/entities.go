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
func (e *entitiesClient) Stream(ctx context.Context, entity, snapshotID, etag string, from, to time.Time) (NDJSONReader, error) {
	if entity == "" {
		return nil, errors.New("extractor: entity is empty")
	}
	if snapshotID == "" {
		return nil, errors.New("extractor: snapshotID is empty")
	}
	u, err := url.Parse(e.c.BaseURL() + "/v1/" + url.PathEscape(entity))
	if err != nil {
		return nil, fmt.Errorf("extractor: parse url: %w", err)
	}
	q := u.Query()
	q.Set("snapshot_id", snapshotID)
	if !from.IsZero() && !to.IsZero() {
		q.Set("event_date_from", from.UTC().Format(dateLayout))
		q.Set("event_date_to", to.UTC().Format(dateLayout))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("extractor: build entities req: %w", err)
	}
	req.Header.Set("Accept", "application/x-ndjson")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := e.c.Do(ctx, req)
	if err != nil {
		return nil, err //nolint:wrapcheck // already wrapped
	}

	switch resp.StatusCode {
	case http.StatusOK:
		respETag := resp.Header.Get("ETag")
		return newNDJSONReader(resp.Body, respETag), nil
	case http.StatusNotModified:
		drainAndClose(resp)
		return newNDJSONReader(io.NopCloser(emptyReader{}), etag), nil
	case http.StatusNotFound, http.StatusNotImplemented:
		// Source-adapter MVP реализует только подмножество entities (см.
		// internal/features/data_export/handler/not_implemented.go: 501 для
		// 14 master/facts entities вне products + receipt_line). Fiber default
		// 404 — для не-зарегистрированных маршрутов.
		//
		// Трактуем оба статуса как «entity ещё не экспортируется» → пустой stream.
		// populateStaging уже толерантен к пустым/неизвестным entities (см. staging.go).
		drainAndClose(resp)
		return newNDJSONReader(io.NopCloser(emptyReader{}), ""), nil
	default:
		drainAndClose(resp)
		return nil, errorspkg.ErrSourceUnavailable.Wrap(
			fmt.Errorf("unexpected status %d on /v1/%s", resp.StatusCode, entity),
		)
	}
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
