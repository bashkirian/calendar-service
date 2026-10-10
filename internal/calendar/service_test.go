package calendar_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/bashkirian/calendar-service/internal/calendar"
)

var t0 = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func TestService_PublishActive_CountsWhatWasDeliveredBeforeAFailure(t *testing.T) {
	repo := &memRepo{entries: newEntries(150)}
	pub := &spyPublisher{failAfter: 130}

	n, err := newService(repo, pub).PublishActive(t.Context())

	require.ErrorIs(t, err, errPublish)
	require.Equal(t, 130, n)
}

func TestService_PublishUnsent(t *testing.T) {
	repo := &memRepo{entries: newEntries(5)}
	pub := &spyPublisher{failAfter: 3}
	svc := newService(repo, pub)

	n, err := svc.PublishUnsent(t.Context())
	require.ErrorIs(t, err, errPublish)
	require.Equal(t, 3, n)
	require.Len(t, repo.sent, 3, "only delivered entries are marked sent")

	pub.failAfter = 0
	n, err = svc.PublishUnsent(t.Context())
	require.NoError(t, err)
	require.Equal(t, 2, n, "the next run sends only the rest")

	n, err = svc.PublishUnsent(t.Context())
	require.NoError(t, err)
	require.Zero(t, n, "sent entries are published only once")
	require.Len(t, pub.sent, 5)
}

func newService(repo *memRepo, pub *spyPublisher) *calendar.Service {
	return calendar.NewService(repo, pub, func() time.Time { return t0 })
}

func newEntries(n int) []calendar.Entry {
	entries := make([]calendar.Entry, n)
	for i := range entries {
		entries[i], _ = calendar.NewEntry(uuid.Must(uuid.NewV7()), t0, t0.Add(time.Hour), t0)
	}
	return entries
}

// memRepo holds only active entries: the "active" filter is SQL and tested against MySQL.
type memRepo struct {
	entries []calendar.Entry
	sent    map[uuid.UUID]bool
}

func (r *memRepo) Create(context.Context, calendar.Entry) error { return nil }

func (r *memRepo) ListActive(context.Context, time.Time) ([]calendar.Entry, error) {
	return r.entries, nil
}

func (r *memRepo) ListUnsentActive(_ context.Context, _ time.Time, limit int) ([]calendar.Entry, error) {
	var out []calendar.Entry
	for _, e := range r.entries {
		if !r.sent[e.ID] && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *memRepo) MarkSent(_ context.Context, ids []uuid.UUID, _ time.Time) error {
	if r.sent == nil {
		r.sent = map[uuid.UUID]bool{}
	}
	for _, id := range ids {
		r.sent[id] = true
	}
	return nil
}

var errPublish = errors.New("queue unavailable")

// spyPublisher fails after failAfter deliveries (0 = never), like a partial SendMessageBatch failure.
type spyPublisher struct {
	sent      []uuid.UUID
	failAfter int
}

func (p *spyPublisher) Publish(_ context.Context, entries []calendar.Entry) ([]uuid.UUID, error) {
	var delivered []uuid.UUID
	for _, e := range entries {
		if p.failAfter > 0 && len(p.sent) == p.failAfter {
			return delivered, errPublish
		}
		p.sent = append(p.sent, e.ID)
		delivered = append(delivered, e.ID)
	}
	return delivered, nil
}
