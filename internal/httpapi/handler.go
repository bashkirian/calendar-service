// Package httpapi serves the REST API.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/bashkirian/calendar-service/internal/calendar"
)

// publishTimeout stays under the server WriteTimeout so the response can still be written.
const publishTimeout = 25 * time.Second

type Service interface {
	Create(ctx context.Context, start, stop time.Time) (calendar.Entry, error)
	ListActive(ctx context.Context) ([]calendar.Entry, error)
	PublishActive(ctx context.Context) (int, error)
}

type entryDTO struct {
	ID        uuid.UUID `json:"id"`
	StartDate time.Time `json:"startDate"`
	StopDate  time.Time `json:"stopDate"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type handler struct {
	svc Service
	log *slog.Logger
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartDate time.Time `json:"startDate"`
		StopDate  time.Time `json:"stopDate"`
	}
	if !h.decode(w, r, &req) {
		return
	}
	e, err := h.svc.Create(r.Context(), req.StartDate, req.StopDate)
	switch {
	case errors.Is(err, calendar.ErrInvalidEntry):
		writeProblem(w, http.StatusUnprocessableEntity, err.Error())
	case err != nil:
		h.fail(w, r, err)
	default:
		writeJSON(w, http.StatusCreated, toDTO(e))
	}
}

func (h *handler) listActive(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.ListActive(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]entryDTO, len(entries))
	for i, e := range entries {
		items[i] = toDTO(e)
	}
	writeJSON(w, http.StatusOK, map[string][]entryDTO{"items": items})
}

// publish is synchronous for simplicity; at scale: 202 and a background job.
func (h *handler) publish(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), publishTimeout)
	defer cancel()
	n, err := h.svc.PublishActive(ctx)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"published": n})
}

func toDTO(e calendar.Entry) entryDTO {
	return entryDTO{
		ID:        e.ID,
		StartDate: e.StartDate,
		StopDate:  e.StopDate,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}
