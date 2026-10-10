package calendar_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/bashkirian/calendar-service/internal/calendar"
)

func TestNewEntry(t *testing.T) {
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	tests := map[string]struct {
		start, stop time.Time
		wantErr     bool
	}{
		"valid":             {start: start, stop: start.Add(time.Hour)},
		"zero length":       {start: start, stop: start},
		"missing start":     {stop: start, wantErr: true},
		"missing stop":      {start: start, wantErr: true},
		"stop before start": {start: start, stop: start.Add(-time.Millisecond), wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := calendar.NewEntry(uuid.Must(uuid.NewV7()), tt.start, tt.stop, start)
			if tt.wantErr {
				require.ErrorIs(t, err, calendar.ErrInvalidEntry)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
