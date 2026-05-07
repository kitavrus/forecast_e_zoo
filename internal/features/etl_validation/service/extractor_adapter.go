package service

import (
	"context"

	"github.com/Kitavrus/e_zoo/internal/features/etl_validation/extractor"
)

// ExtractorAdapter оборачивает SnapshotsClient + EntitiesClient в единый Extractor.
type ExtractorAdapter struct {
	Snapshots extractor.SnapshotsClient
	Entities  extractor.EntitiesClient
}

// GetCurrentSnapshot делегирует SnapshotsClient.
func (a ExtractorAdapter) GetCurrentSnapshot(ctx context.Context) (extractor.Snapshot, error) {
	return a.Snapshots.GetCurrent(ctx) //nolint:wrapcheck // already wrapped via errorspkg
}

// StreamEntity делегирует EntitiesClient.
func (a ExtractorAdapter) StreamEntity(ctx context.Context, entity, snapshotID, etag string) (extractor.NDJSONReader, error) {
	return a.Entities.Stream(ctx, entity, snapshotID, etag) //nolint:wrapcheck // already wrapped
}
