package calendar

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, e Entry) error
	// ListActive returns entries with StopDate >= now, ordered by (StopDate, ID).
	ListActive(ctx context.Context, now time.Time) ([]Entry, error)
	ListUnsentActive(ctx context.Context, now time.Time, limit int) ([]Entry, error)
	MarkSent(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

type Publisher interface {
	// Publish returns the delivered IDs, also when it fails part-way.
	Publish(ctx context.Context, entries []Entry) ([]uuid.UUID, error)
}
