package httpapi_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/bashkirian/calendar-service/internal/calendar"
	"github.com/bashkirian/calendar-service/internal/httpapi"
)

var entry = calendar.Entry{
	ID:        uuid.MustParse("01a12592-1a07-73a1-bb92-7e2480e42fa6"),
	StartDate: time.Date(2026, 10, 12, 7, 0, 0, 0, time.UTC),
	StopDate:  time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC),
	CreatedAt: time.Date(2026, 10, 10, 11, 28, 14, 323e6, time.UTC),
	UpdatedAt: time.Date(2026, 10, 10, 11, 28, 14, 323e6, time.UTC),
}

const entryJSON = `{"id":"01a12592-1a07-73a1-bb92-7e2480e42fa6","startDate":"2026-10-12T07:00:00Z",` +
	`"stopDate":"2026-10-12T09:30:00Z","createdAt":"2026-10-10T11:28:14.323Z","updatedAt":"2026-10-10T11:28:14.323Z"}`

func TestCreateCalendarEntry(t *testing.T) {
	const valid = `{"startDate":"2026-10-12T09:00:00+02:00","stopDate":"2026-10-12T11:30:00+02:00"}`
	tests := map[string]struct {
		body       string
		svcErr     error
		wantStatus int
		wantBody   string
	}{
		"created": {
			body:       valid,
			wantStatus: http.StatusCreated,
			wantBody:   entryJSON,
		},
		"validation error": {
			body:       valid,
			svcErr:     fmt.Errorf("%w: stop date is before start date", calendar.ErrInvalidEntry),
			wantStatus: http.StatusUnprocessableEntity,
			wantBody: `{"type":"about:blank","title":"Unprocessable Entity","status":422,` +
				`"detail":"invalid calendar entry: stop date is before start date"}`,
		},
		"malformed body hides decoder details": {
			body:       `{"startDate":42}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"type":"about:blank","title":"Bad Request","status":400,"detail":"malformed request body"}`,
		},
		"internal error hides details": {
			body:       valid,
			svcErr:     errors.New("dial tcp 10.0.0.1:3306: connection refused"),
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"type":"about:blank","title":"Internal Server Error","status":500}`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resp := do(t, &fakeService{entry: entry, err: tt.svcErr}, http.MethodPost, "/v1/calendar-entries", tt.body)

			require.Equal(t, tt.wantStatus, resp.status)
			if tt.wantStatus != http.StatusCreated {
				require.Equal(t, "application/problem+json", resp.contentType)
			}
			require.JSONEq(t, tt.wantBody, resp.body)
		})
	}
}

func TestListActiveCalendarEntries(t *testing.T) {
	resp := do(t, &fakeService{entries: []calendar.Entry{entry}}, http.MethodGet, "/v1/calendar-entries/active", "")

	require.Equal(t, http.StatusOK, resp.status)
	require.JSONEq(t, `{"items":[`+entryJSON+`]}`, resp.body)
}

func TestPublishActiveCalendarEntries_OutlivesClientDisconnect(t *testing.T) {
	svc := &fakeService{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // client already gone

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/calendar-entries/active/publish", nil)
	httpapi.NewRouter(svc, nil, slog.New(slog.DiscardHandler)).ServeHTTP(httptest.NewRecorder(), req)

	require.NoError(t, svc.publishCtxErr, "publishing must not be cut short by the client")
}

type response struct {
	status      int
	contentType string
	body        string
}

func do(t *testing.T, svc httpapi.Service, method, target, body string) response {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
	httpapi.NewRouter(svc, nil, slog.New(slog.DiscardHandler)).ServeHTTP(rec, req)
	b, err := io.ReadAll(rec.Body)
	require.NoError(t, err)
	return response{status: rec.Code, contentType: rec.Header().Get("Content-Type"), body: string(b)}
}

type fakeService struct {
	entry         calendar.Entry
	entries       []calendar.Entry
	err           error
	publishCtxErr error
}

func (f *fakeService) Create(context.Context, time.Time, time.Time) (calendar.Entry, error) {
	return f.entry, f.err
}

func (f *fakeService) ListActive(context.Context) ([]calendar.Entry, error) {
	return f.entries, f.err
}

func (f *fakeService) PublishActive(ctx context.Context) (int, error) {
	f.publishCtxErr = ctx.Err()
	return 0, f.err
}
