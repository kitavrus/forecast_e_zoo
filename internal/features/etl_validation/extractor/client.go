package extractor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// ClientConfig — параметры HTTP-клиента к source-adapter.
type ClientConfig struct {
	BaseURL     string
	HTTPTimeout time.Duration
	RetryMax    int
	BackoffCap  time.Duration
}

// Defaults для backoff.
const (
	defaultRetryMax   = 3
	defaultBackoffCap = 30 * time.Second
	initialBackoff    = 100 * time.Millisecond
	backoffMultiplier = 2
)

// Client — обёртка над *http.Client с retry/backoff/JWT auth.
type Client struct {
	httpc      *http.Client
	tokenSrc   TokenSource
	cfg        ClientConfig
	logger     *slog.Logger
	sleep      func(d time.Duration) // тестируемая sleep-функция
}

// NewClient собирает Client.
func NewClient(httpc *http.Client, ts TokenSource, cfg ClientConfig, log *slog.Logger) (*Client, error) {
	if httpc == nil {
		return nil, errors.New("extractor: http client is nil")
	}
	if ts == nil {
		return nil, errors.New("extractor: token source is nil")
	}
	if cfg.BaseURL == "" {
		return nil, errors.New("extractor: base URL is empty")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = defaultRetryMax
	}
	if cfg.BackoffCap <= 0 {
		cfg.BackoffCap = defaultBackoffCap
	}
	if log == nil {
		log = slog.Default()
	}
	return &Client{
		httpc:    httpc,
		tokenSrc: ts,
		cfg:      cfg,
		logger:   log,
		sleep:    time.Sleep,
	}, nil
}

// BaseURL возвращает baseURL без trailing slash.
func (c *Client) BaseURL() string { return c.cfg.BaseURL }

// Do добавляет Authorization, шлёт запрос с retry на 5xx/networking.
//
// 4xx (за исключением 429) не retry-ит — возвращает ответ как есть.
// После исчерпания retry — *errorspkg.Error (ErrSourceUnavailable) с обёрткой.
//
// Caller обязан закрыть resp.Body, если err == nil.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("extractor: request is nil")
	}
	tok, err := c.tokenSrc.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("extractor: token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}

	var lastErr error
	backoff := initialBackoff
	for attempt := 0; attempt <= c.cfg.RetryMax; attempt++ {
		if attempt > 0 {
			c.logger.WarnContext(ctx, "extractor: retry",
				"attempt", attempt, "url", req.URL.String(), "backoff", backoff)
			c.sleep(backoff)
			backoff *= backoffMultiplier
			if backoff > c.cfg.BackoffCap {
				backoff = c.cfg.BackoffCap
			}
		}
		// Important: re-do request must use a fresh request (Body reset).
		// Caller is expected to use no-body or io.NopCloser(bytes.NewReader(...)).
		// URL формируется из baseURL (env-var ETL_SOURCE_ADAPTER_URL), он внутренний
		// для деплоя и не подвержен SSRF от пользовательского ввода.
		resp, doErr := c.httpc.Do(req.Clone(ctx)) //nolint:gosec // see comment
		if doErr != nil {
			lastErr = doErr
			if errors.Is(doErr, context.Canceled) || errors.Is(doErr, context.DeadlineExceeded) {
				return nil, fmt.Errorf("extractor: request canceled: %w", doErr)
			}
			continue
		}
		if resp.StatusCode >= http.StatusInternalServerError ||
			resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("status=%d", resp.StatusCode)
			drainAndClose(resp)
			continue
		}
		return resp, nil
	}
	return nil, errorspkg.ErrSourceUnavailable.Wrap(
		fmt.Errorf("retry exhausted (max=%d): %w", c.cfg.RetryMax, lastErr),
	)
}

func drainAndClose(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}
