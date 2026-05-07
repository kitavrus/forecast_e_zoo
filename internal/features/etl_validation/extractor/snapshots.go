package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Kitavrus/e_zoo/pkg/errorspkg"
)

// Snapshot — DTO ответа GET /v1/snapshots/current от source-adapter.
type Snapshot struct {
	LoadID    string    `json:"load_id"`
	CreatedAt time.Time `json:"created_at"`
	ETag      string    `json:"etag,omitempty"`
}

// SnapshotsClient — публичный интерфейс для тестов EtlPipeline.
type SnapshotsClient interface {
	GetCurrent(ctx context.Context) (Snapshot, error)
}

type snapshotsClient struct {
	c *Client
}

// NewSnapshotsClient оборачивает Client.
func NewSnapshotsClient(c *Client) SnapshotsClient {
	return &snapshotsClient{c: c}
}

// GetCurrent шлёт GET /v1/snapshots/current.
//
// HTTP 503 → errorspkg.ErrSnapshotNotReady.
func (s *snapshotsClient) GetCurrent(ctx context.Context) (Snapshot, error) {
	url := s.c.BaseURL() + "/v1/snapshots/current"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("extractor: build snapshots req: %w", err)
	}
	resp, err := s.c.Do(ctx, req)
	if err != nil {
		return Snapshot{}, err //nolint:wrapcheck // already wrapped via errorspkg
	}
	defer drainAndClose(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		var snap Snapshot
		if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
			return Snapshot{}, fmt.Errorf("extractor: decode snapshot: %w", err)
		}
		if snap.ETag == "" {
			snap.ETag = resp.Header.Get("ETag")
		}
		return snap, nil
	case http.StatusServiceUnavailable:
		return Snapshot{}, errorspkg.ErrSnapshotNotReady.Wrap(
			fmt.Errorf("source returned 503"),
		)
	default:
		return Snapshot{}, errorspkg.ErrSourceUnavailable.Wrap(
			fmt.Errorf("unexpected status %d on snapshots/current", resp.StatusCode),
		)
	}
}
