// Package sqs publishes and consumes calendar entries over SQS.
package sqs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bashkirian/calendar-service/internal/calendar"
)

// Breaking payload changes bump schemaVersion; consumers reject unknown versions.
const (
	eventType     = "calendar.entry.active"
	schemaVersion = "1"
)

// envelope is the wire format; consumers deduplicate by data.id.
type envelope struct {
	Type       string    `json:"type"`
	Version    string    `json:"version"`
	OccurredAt time.Time `json:"occurredAt"`
	Data       entryV1   `json:"data"`
}

type entryV1 struct {
	ID        uuid.UUID `json:"id"`
	StartDate time.Time `json:"startDate"`
	StopDate  time.Time `json:"stopDate"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func encode(e calendar.Entry, now time.Time) (string, error) {
	body, err := json.Marshal(envelope{
		Type:       eventType,
		Version:    schemaVersion,
		OccurredAt: now.UTC(),
		Data: entryV1{
			ID:        e.ID,
			StartDate: e.StartDate,
			StopDate:  e.StopDate,
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
		},
	})
	return string(body), err
}

func decode(body string) (calendar.Entry, error) {
	var env envelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		return calendar.Entry{}, fmt.Errorf("unmarshal envelope: %w", err)
	}
	if env.Type != eventType || env.Version != schemaVersion {
		return calendar.Entry{}, fmt.Errorf("unsupported message %s v%s", env.Type, env.Version)
	}
	d := env.Data
	return calendar.Entry{
		ID:        d.ID,
		StartDate: d.StartDate,
		StopDate:  d.StopDate,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}, nil
}
