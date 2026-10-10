package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const publishBatchSize = 100

type Service struct {
	repo Repository
	pub  Publisher
	now  func() time.Time
}

func NewService(repo Repository, pub Publisher, now func() time.Time) *Service {
	return &Service{repo: repo, pub: pub, now: now}
}

func (s *Service) Create(ctx context.Context, start, stop time.Time) (Entry, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Entry{}, fmt.Errorf("generate id: %w", err)
	}
	e, err := NewEntry(id, start, stop, s.now())
	if err != nil {
		return Entry{}, err
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return Entry{}, fmt.Errorf("store entry: %w", err)
	}
	return e, nil
}

func (s *Service) ListActive(ctx context.Context) ([]Entry, error) {
	entries, err := s.repo.ListActive(ctx, s.now())
	if err != nil {
		return nil, fmt.Errorf("list active entries: %w", err)
	}
	return entries, nil
}

// PublishActive resends everything on each call.
func (s *Service) PublishActive(ctx context.Context) (int, error) {
	entries, err := s.ListActive(ctx)
	if err != nil {
		return 0, err
	}
	sent, err := s.pub.Publish(ctx, entries)
	if err != nil {
		return len(sent), fmt.Errorf("publish entries: %w", err)
	}
	return len(sent), nil
}

// PublishUnsent sends each entry once, best effort; assumes one runner (the scheduler guard).
func (s *Service) PublishUnsent(ctx context.Context) (int, error) {
	now := s.now()
	total := 0
	for {
		entries, err := s.repo.ListUnsentActive(ctx, now, publishBatchSize)
		if err != nil {
			return total, fmt.Errorf("list unsent entries: %w", err)
		}
		if len(entries) == 0 {
			return total, nil
		}
		sent, pubErr := s.pub.Publish(ctx, entries)
		if len(sent) > 0 {
			if err := s.repo.MarkSent(ctx, sent, now); err != nil {
				return total, fmt.Errorf("mark sent: %w", err)
			}
		}
		total += len(sent)
		if pubErr != nil {
			return total, fmt.Errorf("publish entries: %w", pubErr)
		}
		if len(entries) < publishBatchSize {
			return total, nil
		}
	}
}
