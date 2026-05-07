// Package sender — ErpAPISender: HTTP client с retry/backoff для erp_api channel.
package sender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Kitavrus/e_zoo/internal/features/channels/auth"
	"github.com/Kitavrus/e_zoo/internal/features/channels/constants"
	"github.com/Kitavrus/e_zoo/internal/features/channels/formatter"
	"github.com/Kitavrus/e_zoo/internal/features/channels/models"
)

// HTTPClient — узкий интерфейс над *http.Client (для тестов).
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Defaults для backoff (зеркалит etl_validation/extractor).
const (
	defaultRetryMax    = 3
	defaultBackoffCap  = 30 * time.Second
	initialBackoff     = 200 * time.Millisecond
	backoffMultiplier  = 2
	defaultHTTPTimeout = 30 * time.Second
	maxBodyDumpBytes   = 8 * 1024 // 8 KiB — лимит для request/response_body в audit
)

// ErpAPISender — REST API клиент к ERP клиенту (POST endpoint_url).
type ErpAPISender struct {
	httpc        HTTPClient
	authProvider auth.Provider
	formatter    formatter.Formatter
	logger       *slog.Logger
	sleep        func(d time.Duration)
}

// Config — параметры ErpAPISender.
type Config struct {
	HTTPClient   HTTPClient
	AuthProvider auth.Provider
	Formatter    formatter.Formatter
	Logger       *slog.Logger
}

// NewErpAPISender создаёт sender.
func NewErpAPISender(cfg Config) (*ErpAPISender, error) {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	if cfg.AuthProvider == nil {
		return nil, errors.New("sender/erp_api: auth provider is nil")
	}
	if cfg.Formatter == nil {
		return nil, errors.New("sender/erp_api: formatter is nil")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &ErpAPISender{
		httpc:        cfg.HTTPClient,
		authProvider: cfg.AuthProvider,
		formatter:    cfg.Formatter,
		logger:       cfg.Logger,
		sleep:        time.Sleep,
	}, nil
}

// ChannelType возвращает constants.ChannelTypeErpAPI.
func (s *ErpAPISender) ChannelType() string { return constants.ChannelTypeErpAPI }

// Send отправляет PO с retry/backoff. См. ADR-003.
//
//nolint:funlen,cyclop,gocognit // централизованная логика retry+audit; разбивать вредно для читаемости
func (s *ErpAPISender) Send(
	ctx context.Context, in SendInput, cfg models.SupplierChannelConfig,
) (SendResult, error) {
	startedAt := time.Now().UTC()
	res := SendResult{StartedAt: startedAt}

	// 1. Format body.
	payload := formatter.PurchaseOrderPayload{PO: in.PO, Lines: in.Lines}
	body, contentType, err := s.formatter.Format(payload)
	if err != nil {
		res.Status = constants.SendAttemptStatusFailed
		res.ErrorMessage = ptrStr(fmt.Sprintf("format: %s", err.Error()))
		now := time.Now().UTC()
		res.FinishedAt = &now
		return res, fmt.Errorf("sender/erp_api: format: %w", err)
	}
	bodyDump := truncateForAudit(body, maxBodyDumpBytes)
	res.RequestBody = ptrStr(bodyDump)

	// 2. Determine retry policy.
	retryMax := cfg.RetryMax
	if retryMax <= 0 {
		retryMax = defaultRetryMax
	}

	endpoint := strings.TrimRight(cfg.EndpointURL, "/")
	if endpoint == "" {
		res.Status = constants.SendAttemptStatusFailed
		res.ErrorMessage = ptrStr("empty endpoint_url")
		now := time.Now().UTC()
		res.FinishedAt = &now
		return res, errors.New("sender/erp_api: empty endpoint_url")
	}

	// 3. Retry loop.
	backoff := initialBackoff
	var lastErr error
	for attempt := 0; attempt <= retryMax; attempt++ {
		if attempt > 0 {
			s.logger.WarnContext(ctx, "channels/erp_api: retry",
				slog.Int("attempt", attempt),
				slog.String("po_number", in.PO.PONumber),
				slog.Duration("backoff", backoff),
			)
			s.sleep(backoff)
			backoff *= backoffMultiplier
			if backoff > defaultBackoffCap {
				backoff = defaultBackoffCap
			}
			res.RetryCount = attempt
		}

		req, reqErr := http.NewRequestWithContext(
			ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if reqErr != nil {
			lastErr = fmt.Errorf("new request: %w", reqErr)
			continue
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Idempotency-Key", in.PO.PONumber)
		req.Header.Set("Accept", "application/json")
		if applyErr := s.authProvider.Apply(ctx, req, cfg); applyErr != nil {
			lastErr = fmt.Errorf("auth apply: %w", applyErr)
			res.Status = constants.SendAttemptStatusFailed
			res.ErrorMessage = ptrStr(lastErr.Error())
			now := time.Now().UTC()
			res.FinishedAt = &now
			return res, fmt.Errorf("sender/erp_api: %w", lastErr)
		}

		resp, doErr := s.httpc.Do(req) //nolint:bodyclose // body закрывается ниже через handleResponse
		if doErr != nil {
			lastErr = fmt.Errorf("http do: %w", doErr)
			s.logger.WarnContext(ctx, "channels/erp_api: network error",
				slog.String("error", doErr.Error()))
			continue // network error — retry
		}

		shouldRetry, finalErr := s.handleResponse(resp, &res)
		if !shouldRetry {
			now := time.Now().UTC()
			res.FinishedAt = &now
			if finalErr != nil {
				return res, fmt.Errorf("sender/erp_api: %w", finalErr)
			}
			return res, nil
		}
		lastErr = finalErr
	}

	// 4. Retry exhausted.
	res.Status = constants.SendAttemptStatusFailed
	if lastErr != nil {
		res.ErrorMessage = ptrStr(fmt.Sprintf("retry exhausted: %s", lastErr.Error()))
	} else {
		res.ErrorMessage = ptrStr("retry exhausted")
	}
	now := time.Now().UTC()
	res.FinishedAt = &now
	if lastErr == nil {
		lastErr = errors.New("retry exhausted")
	}
	return res, fmt.Errorf("sender/erp_api: %w", lastErr)
}

// handleResponse обрабатывает HTTP-ответ.
//
// Возвращает (shouldRetry, error).
//   - 2xx → success, shouldRetry=false, err=nil.
//   - 4xx (кроме 429) → failed, shouldRetry=false (4xx — клиентская ошибка).
//   - 5xx, 429 → shouldRetry=true.
func (s *ErpAPISender) handleResponse(resp *http.Response, res *SendResult) (bool, error) {
	defer func() { _ = resp.Body.Close() }()
	res.HTTPStatusCode = ptrInt(resp.StatusCode)

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyDumpBytes+1))
	res.ResponseBody = ptrStr(truncateForAudit(respBody, maxBodyDumpBytes))

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300: //nolint:mnd
		res.Status = constants.SendAttemptStatusSuccess
		ext := extractExternalRef(respBody)
		if ext != "" {
			res.ExternalRef = ptrStr(ext)
		}
		return false, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500: //nolint:mnd
		// retry
		res.Status = constants.SendAttemptStatusPending
		return true, fmt.Errorf("http %d", resp.StatusCode)
	default: //nolint:mnd
		// 4xx (но не 429) — клиентская ошибка, не retry-им.
		res.Status = constants.SendAttemptStatusFailed
		res.ErrorMessage = ptrStr(fmt.Sprintf("http %d: %s",
			resp.StatusCode, truncateForAudit(respBody, 256)))
		return false, fmt.Errorf("http %d", resp.StatusCode)
	}
}

// truncateForAudit обрезает body до n байт. Возвращает строку.
func truncateForAudit(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...[truncated]"
}

// extractExternalRef пытается достать external_ref из ответа.
//
// MVP: поддержка ключей "id", "external_ref", "po_number" в плоском JSON.
// Если ничего не найдено — возвращает "".
func extractExternalRef(body []byte) string {
	// очень дешёвая extraction без полного parse — поиск по подстрокам.
	// Реальный JSON-parse сделает service-уровень при необходимости.
	keys := [...]string{`"external_ref":"`, `"id":"`, `"po_number":"`}
	for _, k := range keys {
		idx := bytes.Index(body, []byte(k))
		if idx < 0 {
			continue
		}
		start := idx + len(k)
		end := bytes.IndexByte(body[start:], '"')
		if end < 0 {
			continue
		}
		return string(body[start : start+end])
	}
	return ""
}
