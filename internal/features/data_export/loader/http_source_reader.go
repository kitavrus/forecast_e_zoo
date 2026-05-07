package loader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HTTPSourceReader — реальная реализация SourceReader, ходящая в mock-erp/E-Zoo
// REST по контракту:
//
//	Master: GET {BaseURL}/api/v1/{entity}?since=<ISO-8601>&cursor=<opaque>&limit=<n>
//	Facts:  GET {BaseURL}/api/v1/{entity}?date_from=<YYYY-MM-DD>&date_to=<YYYY-MM-DD>&cursor=<opaque>&limit=<n>
//
// Pagination: пока ответ содержит непустой X-Next-Cursor — продолжаем читать.
// Auth: X-API-Key, если cfg.APIKey не пустой.
// Retry: 5xx и 429 → exponential backoff (200ms, 400ms, ... cap), max RetryMax попыток.
// 4xx (кроме 429) — сразу ошибка без retry.
//
// Все строки одной сущности накапливаются в slice до возврата итератора —
// упрощение MVP; смена на streaming-итератор не меняет SourceReader контракт.

// HTTPSourceReaderConfig — параметры конструктора HTTPSourceReader.
type HTTPSourceReaderConfig struct {
	BaseURL      string        // например, "http://mock-erp:8090"
	APIKey       string        // optional, отправляется в заголовке X-API-Key
	HTTPTimeout  time.Duration // default 30s
	RetryMax     int           // default 3
	BackoffCap   time.Duration // default 30s
	RequestLimit int           // default 10000 (page size, query &limit=)
	Logger       *slog.Logger  // optional, slog.Default() если nil
}

// HTTPSourceReader реализует SourceReader через HTTP REST.
type HTTPSourceReader struct {
	httpc      *http.Client
	cfg        HTTPSourceReaderConfig
	log        *slog.Logger
	sleepFn    func(time.Duration) // тестируемая sleep
	dateFormat string              // YYYY-MM-DD для facts
}

// Defaults для HTTPSourceReader.
const (
	httpDefaultTimeout      = 30 * time.Second
	httpDefaultRetryMax     = 3
	httpDefaultBackoffCap   = 30 * time.Second
	httpDefaultRequestLimit = 10000
	httpInitialBackoff      = 200 * time.Millisecond
	httpBackoffMultiplier   = 2
)

// NewHTTPSourceReader — конструктор.
func NewHTTPSourceReader(cfg HTTPSourceReaderConfig) *HTTPSourceReader {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = httpDefaultTimeout
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = httpDefaultRetryMax
	}
	if cfg.BackoffCap <= 0 {
		cfg.BackoffCap = httpDefaultBackoffCap
	}
	if cfg.RequestLimit <= 0 {
		cfg.RequestLimit = httpDefaultRequestLimit
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &HTTPSourceReader{
		httpc:      &http.Client{Timeout: cfg.HTTPTimeout},
		cfg:        cfg,
		log:        cfg.Logger,
		sleepFn:    time.Sleep,
		dateFormat: "2006-01-02",
	}
}

// Close закрывает читатель (idle-keepalive). Errors — никогда.
func (r *HTTPSourceReader) Close(_ context.Context) error {
	r.httpc.CloseIdleConnections()
	return nil
}

// fetchAll — общий обход endpoint c курсорной пагинацией.
// path — без leading slash (например, "products").
// queryBuilder заполняет url.Values до cursor/limit (since или date_from/date_to).
// out — указатель на []T, в который десериализуется тело ответа на каждой странице.
func fetchAll[T any](
	ctx context.Context,
	r *HTTPSourceReader,
	path string,
	queryBuilder func(url.Values),
) ([]T, error) {
	endpoint := r.cfg.BaseURL + "/api/v1/" + path
	cursor := ""
	all := make([]T, 0, r.cfg.RequestLimit)
	for {
		q := url.Values{}
		if queryBuilder != nil {
			queryBuilder(q)
		}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		q.Set("limit", strconv.Itoa(r.cfg.RequestLimit))

		page, next, err := doRequest[T](ctx, r, endpoint, q)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			break
		}
		cursor = next
	}
	return all, nil
}

// doRequest — один HTTP-запрос с retry на 5xx/429. Возвращает (rows, nextCursor, err).
func doRequest[T any](
	ctx context.Context,
	r *HTTPSourceReader,
	endpoint string,
	q url.Values,
) ([]T, string, error) {
	full := endpoint
	if encoded := q.Encode(); encoded != "" {
		full = endpoint + "?" + encoded
	}

	var lastErr error
	backoff := httpInitialBackoff
	for attempt := 0; attempt <= r.cfg.RetryMax; attempt++ {
		if attempt > 0 {
			r.log.WarnContext(ctx, "http_source_reader: retry",
				"attempt", attempt, "url", full, "backoff", backoff)
			r.sleepFn(backoff)
			backoff *= httpBackoffMultiplier
			if backoff > r.cfg.BackoffCap {
				backoff = r.cfg.BackoffCap
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, http.NoBody)
		if err != nil {
			return nil, "", fmt.Errorf("http_source_reader: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if r.cfg.APIKey != "" {
			req.Header.Set("X-API-Key", r.cfg.APIKey)
		}

		resp, doErr := r.httpc.Do(req)
		if doErr != nil {
			lastErr = doErr
			if errors.Is(doErr, context.Canceled) || errors.Is(doErr, context.DeadlineExceeded) {
				return nil, "", fmt.Errorf("http_source_reader: request canceled: %w", doErr)
			}
			continue
		}

		// 5xx / 429 — retry.
		if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("status=%d", resp.StatusCode)
			drainAndCloseHTTP(resp)
			continue
		}

		// 4xx (не 429) — без retry.
		if resp.StatusCode >= http.StatusBadRequest {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = resp.Body.Close()
			return nil, "", fmt.Errorf("http_source_reader: %s status=%d body=%q", endpoint, resp.StatusCode, string(body))
		}

		// 2xx.
		var rows []T
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&rows); err != nil {
			_ = resp.Body.Close()
			return nil, "", fmt.Errorf("http_source_reader: decode %s: %w", endpoint, err)
		}
		nextCursor := resp.Header.Get("X-Next-Cursor")
		_ = resp.Body.Close()
		return rows, nextCursor, nil
	}

	return nil, "", fmt.Errorf("http_source_reader: retry exhausted (max=%d) for %s: %w", r.cfg.RetryMax, endpoint, lastErr)
}

func drainAndCloseHTTP(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

// masterQuery возвращает builder для query-параметров master entity (?since=).
func (r *HTTPSourceReader) masterQuery(since time.Time) func(url.Values) {
	return func(q url.Values) {
		if !since.IsZero() {
			q.Set("since", since.UTC().Format(time.RFC3339))
		}
	}
}

// factsQuery возвращает builder для query-параметров facts entity (?date_from=&date_to=).
func (r *HTTPSourceReader) factsQuery(dateFrom, dateTo time.Time) func(url.Values) {
	return func(q url.Values) {
		if !dateFrom.IsZero() {
			q.Set("date_from", dateFrom.UTC().Format(r.dateFormat))
		}
		if !dateTo.IsZero() {
			q.Set("date_to", dateTo.UTC().Format(r.dateFormat))
		}
	}
}

// --- master readers ---

// ReadProducts читает master-сущность products.
func (r *HTTPSourceReader) ReadProducts(ctx context.Context, since time.Time) (PageIterator[ErpProduct], error) {
	rows, err := fetchAll[ErpProduct](ctx, r, "products", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadProductBarcodes читает master-сущность product_barcodes.
func (r *HTTPSourceReader) ReadProductBarcodes(ctx context.Context, since time.Time) (PageIterator[ErpProductBarcode], error) {
	rows, err := fetchAll[ErpProductBarcode](ctx, r, "product_barcodes", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadCategory читает master-сущность category.
func (r *HTTPSourceReader) ReadCategory(ctx context.Context, since time.Time) (PageIterator[ErpCategory], error) {
	rows, err := fetchAll[ErpCategory](ctx, r, "category", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadLocation читает master-сущность location.
func (r *HTTPSourceReader) ReadLocation(ctx context.Context, since time.Time) (PageIterator[ErpLocation], error) {
	rows, err := fetchAll[ErpLocation](ctx, r, "location", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadSupplier читает master-сущность supplier.
func (r *HTTPSourceReader) ReadSupplier(ctx context.Context, since time.Time) (PageIterator[ErpSupplier], error) {
	rows, err := fetchAll[ErpSupplier](ctx, r, "supplier", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadSupplySpec читает master-сущность supply_spec.
func (r *HTTPSourceReader) ReadSupplySpec(ctx context.Context, since time.Time) (PageIterator[ErpSupplySpec], error) {
	rows, err := fetchAll[ErpSupplySpec](ctx, r, "supply_spec", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadPromo читает master-сущность promo.
func (r *HTTPSourceReader) ReadPromo(ctx context.Context, since time.Time) (PageIterator[ErpPromo], error) {
	rows, err := fetchAll[ErpPromo](ctx, r, "promo", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadOrderRule читает master-сущность order_rule.
func (r *HTTPSourceReader) ReadOrderRule(ctx context.Context, since time.Time) (PageIterator[ErpOrderRule], error) {
	rows, err := fetchAll[ErpOrderRule](ctx, r, "order_rule", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadSupplyPlan читает master-сущность supply_plan.
func (r *HTTPSourceReader) ReadSupplyPlan(ctx context.Context, since time.Time) (PageIterator[ErpSupplyPlan], error) {
	rows, err := fetchAll[ErpSupplyPlan](ctx, r, "supply_plan", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadStoreAssortment читает master-сущность store_assortment.
func (r *HTTPSourceReader) ReadStoreAssortment(ctx context.Context, since time.Time) (PageIterator[ErpStoreAssortment], error) {
	rows, err := fetchAll[ErpStoreAssortment](ctx, r, "store_assortment", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadStoreAssortmentLifecycleEvents читает master-сущность store_assortment_lifecycle_events.
func (r *HTTPSourceReader) ReadStoreAssortmentLifecycleEvents(ctx context.Context, since time.Time) (PageIterator[ErpStoreAssortmentLifecycleEvent], error) {
	rows, err := fetchAll[ErpStoreAssortmentLifecycleEvent](ctx, r, "store_assortment_lifecycle_events", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadMasterChangeLog читает master-сущность master_change_log.
func (r *HTTPSourceReader) ReadMasterChangeLog(ctx context.Context, since time.Time) (PageIterator[ErpMasterChangeLog], error) {
	rows, err := fetchAll[ErpMasterChangeLog](ctx, r, "master_change_log", r.masterQuery(since))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// --- facts readers ---

// ReadReceiptLine читает facts-сущность receipt_line.
func (r *HTTPSourceReader) ReadReceiptLine(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpReceiptLine], error) {
	rows, err := fetchAll[ErpReceiptLine](ctx, r, "receipt_line", r.factsQuery(dateFrom, dateTo))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadLocationStockSnapshot читает facts-сущность location_stock_snapshot.
func (r *HTTPSourceReader) ReadLocationStockSnapshot(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpLocationStockSnapshot], error) {
	rows, err := fetchAll[ErpLocationStockSnapshot](ctx, r, "location_stock_snapshot", r.factsQuery(dateFrom, dateTo))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadStockMovement читает facts-сущность stock_movement.
func (r *HTTPSourceReader) ReadStockMovement(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpStockMovement], error) {
	rows, err := fetchAll[ErpStockMovement](ctx, r, "stock_movement", r.factsQuery(dateFrom, dateTo))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}

// ReadSupplierStockSnapshot читает facts-сущность supplier_stock_snapshot.
func (r *HTTPSourceReader) ReadSupplierStockSnapshot(ctx context.Context, dateFrom, dateTo time.Time) (PageIterator[ErpSupplierStockSnapshot], error) {
	rows, err := fetchAll[ErpSupplierStockSnapshot](ctx, r, "supplier_stock_snapshot", r.factsQuery(dateFrom, dateTo))
	if err != nil {
		return nil, err
	}
	return newSliceIterator(rows), nil
}
