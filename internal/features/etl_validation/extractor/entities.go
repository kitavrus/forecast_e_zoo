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

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// NDJSONScannerBufSize — максимальный размер одной NDJSON-строки.
const NDJSONScannerBufSize = 1 << 20 // 1 MiB

// EntitiesClient — публичный интерфейс для тестов EtlPipeline.
type EntitiesClient interface {
	Stream(ctx context.Context, entity, snapshotID, etag string) (NDJSONReader, error)
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

// Stream открывает поток GET /v1/entities/{entity}?snapshot_id=...
// и возвращает NDJSONReader. Если etag совпал → 304 Not Modified, NDJSONReader
// окажется пустым (Next сразу вернёт io.EOF) и ETag() вернёт переданный etag.
func (e *entitiesClient) Stream(ctx context.Context, entity, snapshotID, etag string) (NDJSONReader, error) {
	if entity == "" {
		return nil, errors.New("extractor: entity is empty")
	}
	if snapshotID == "" {
		return nil, errors.New("extractor: snapshotID is empty")
	}
	u, err := url.Parse(e.c.BaseURL() + "/v1/entities/" + url.PathEscape(entity))
	if err != nil {
		return nil, fmt.Errorf("extractor: parse url: %w", err)
	}
	q := u.Query()
	q.Set("snapshot_id", snapshotID)
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
	case http.StatusNotFound:
		drainAndClose(resp)
		return nil, errorspkg.ErrSourceUnavailable.Wrap(
			fmt.Errorf("entity %q not found", entity),
		)
	default:
		drainAndClose(resp)
		return nil, errorspkg.ErrSourceUnavailable.Wrap(
			fmt.Errorf("unexpected status %d on entities/%s", resp.StatusCode, entity),
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
