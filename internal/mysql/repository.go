package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bashkirian/calendar-service/internal/calendar"
	"github.com/bashkirian/calendar-service/internal/mysql/sqlcgen"
)

var _ calendar.Repository = (*Repository)(nil)

type Repository struct {
	q *sqlcgen.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{q: sqlcgen.New(db)}
}

func (r *Repository) Create(ctx context.Context, e calendar.Entry) error {
	return r.q.CreateEntry(ctx, sqlcgen.CreateEntryParams{
		ID:        e.ID[:],
		StartDate: e.StartDate,
		StopDate:  e.StopDate,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	})
}

func (r *Repository) ListActive(ctx context.Context, now time.Time) ([]calendar.Entry, error) {
	rows, err := r.q.ListActive(ctx, now)
	if err != nil {
		return nil, err
	}
	return toEntries(rows)
}

func (r *Repository) ListUnsentActive(ctx context.Context, now time.Time, limit int) ([]calendar.Entry, error) {
	rows, err := r.q.ListUnsentActive(ctx, sqlcgen.ListUnsentActiveParams{
		StopDate: now,
		Limit:    int32(limit), //nolint:gosec // limit is bounded by callers
	})
	if err != nil {
		return nil, err
	}
	return toEntries(rows)
}

func (r *Repository) MarkSent(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	raw := make([][]byte, len(ids))
	for i, id := range ids {
		raw[i] = id[:]
	}
	return r.q.MarkSent(ctx, sqlcgen.MarkSentParams{SentAt: sql.NullTime{Time: at, Valid: true}, Ids: raw})
}

// ClaimRun reports whether the caller won this run of name; one winner per period across replicas.
func (r *Repository) ClaimRun(ctx context.Context, name string, period time.Duration) (bool, error) {
	n, err := r.q.ClaimJobRun(ctx, sqlcgen.ClaimJobRunParams{Name: name, IntervalUs: period.Microseconds()})
	return n > 0, err
}

func toEntries(rows []sqlcgen.CalendarEntry) ([]calendar.Entry, error) {
	entries := make([]calendar.Entry, len(rows))
	for i, row := range rows {
		id, err := uuid.FromBytes(row.ID)
		if err != nil {
			return nil, fmt.Errorf("decode id: %w", err)
		}
		entries[i] = calendar.Entry{
			ID:        id,
			StartDate: row.StartDate,
			StopDate:  row.StopDate,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
	}
	return entries, nil
}
