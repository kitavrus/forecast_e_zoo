package models

import (
	"time"

	"github.com/google/uuid"
)

// SnapshotPointer — указатель текущего consistent snapshot (master+facts).
// Хранится в одной строке snapshot_pointer (id=1).
type SnapshotPointer struct {
	CurrentLoadID  *uuid.UUID `db:"current_load_id" json:"current_load_id,omitempty"`
	PreviousLoadID *uuid.UUID `db:"previous_load_id" json:"previous_load_id,omitempty"`
	CommittedAt    *time.Time `db:"committed_at" json:"committed_at,omitempty"`
}
