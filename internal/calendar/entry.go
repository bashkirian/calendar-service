// Package calendar holds the domain model, use cases and ports.
package calendar

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidEntry = errors.New("invalid calendar entry")

// precision matches DATETIME(3), so values round-trip through storage unchanged.
const precision = time.Millisecond

type Entry struct {
	ID        uuid.UUID
	StartDate time.Time
	StopDate  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewEntry(id uuid.UUID, start, stop, now time.Time) (Entry, error) {
	if start.IsZero() || stop.IsZero() {
		return Entry{}, fmt.Errorf("%w: start and stop dates are required", ErrInvalidEntry)
	}
	start, stop, now = normalize(start), normalize(stop), normalize(now)
	if stop.Before(start) {
		return Entry{}, fmt.Errorf("%w: stop date is before start date", ErrInvalidEntry)
	}
	return Entry{ID: id, StartDate: start, StopDate: stop, CreatedAt: now, UpdatedAt: now}, nil
}

func normalize(t time.Time) time.Time {
	return t.UTC().Truncate(precision)
}
