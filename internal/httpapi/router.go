package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

const maxBodyBytes = 1 << 20

func NewRouter(svc Service, ready func(context.Context) error, log *slog.Logger) http.Handler {
	h := &handler{svc: svc, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/calendar-entries", h.create)
	mux.HandleFunc("GET /v1/calendar-entries/active", h.listActive)
	mux.HandleFunc("POST /v1/calendar-entries/active/publish", h.publish)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := ready(r.Context()); err != nil {
			log.WarnContext(r.Context(), "not ready", "err", err)
			writeProblem(w, http.StatusServiceUnavailable, "dependency unavailable")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

// decode accepts one size-limited JSON object; decoder details are logged, not returned.
func (h *handler) decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	err := dec.Decode(dst)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("trailing data after JSON body")
	}
	if err == nil {
		return true
	}
	h.log.InfoContext(r.Context(), "malformed request body", "err", err)
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		writeProblem(w, http.StatusRequestEntityTooLarge, "request body exceeds 1 MiB")
		return false
	}
	writeProblem(w, http.StatusBadRequest, "malformed request body")
	return false
}

func (h *handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	h.log.ErrorContext(r.Context(), "request failed", "err", err)
	writeProblem(w, http.StatusInternalServerError, "")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeProblem answers with RFC 9457 problem details.
func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail,omitempty"`
	}{"about:blank", http.StatusText(status), status, detail})
}
