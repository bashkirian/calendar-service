package mysql_test

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	"github.com/bashkirian/calendar-service/internal/calendar"
	"github.com/bashkirian/calendar-service/internal/mysql"
)

// The "active" rule lives in SQL, so it is tested against a real MySQL rather than a mock.
var testDB *sql.DB

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if testing.Short() {
		return m.Run()
	}
	ctx := context.Background()
	ctr, err := tcmysql.Run(ctx, "mysql:8.4", tcmysql.WithDatabase("calendar"))
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	if err != nil {
		log.Printf("start mysql container: %v", err)
		return 1
	}
	dsn, err := ctr.ConnectionString(ctx)
	if err == nil {
		testDB, err = mysql.Open(ctx, dsn)
	}
	if err == nil {
		err = mysql.Migrate(ctx, testDB)
	}
	if err != nil {
		log.Printf("prepare database: %v", err)
		return 1
	}
	return m.Run()
}

var now = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func TestListActive(t *testing.T) {
	repo := newRepository(t)
	future := insert(t, repo, now, now.Add(time.Hour))
	ended := insert(t, repo, now.Add(-time.Hour), now.Add(-time.Millisecond)) // smallest step into the past
	endsNow := insert(t, repo, now.Add(-time.Hour), now)                      // boundary: still active

	got, err := repo.ListActive(t.Context(), now)

	require.NoError(t, err)
	require.Equal(t, []calendar.Entry{endsNow, future}, got, "ordered by stop date")
	require.NotContains(t, got, ended)
}

func TestListUnsentActiveAndMarkSent(t *testing.T) {
	ctx := t.Context()
	repo := newRepository(t)
	first := insert(t, repo, now, now.Add(time.Hour))
	second := insert(t, repo, now, now.Add(2*time.Hour))
	insert(t, repo, now.Add(-time.Hour), now.Add(-time.Minute)) // inactive

	got, err := repo.ListUnsentActive(ctx, now, 10)
	require.NoError(t, err)
	require.Equal(t, []calendar.Entry{first, second}, got)

	require.NoError(t, repo.MarkSent(ctx, []uuid.UUID{first.ID}, now))

	got, err = repo.ListUnsentActive(ctx, now, 10)
	require.NoError(t, err)
	require.Equal(t, []calendar.Entry{second}, got, "only the marked entry is excluded")
}

// Regression: GET_LOCK let each phase-shifted replica run once per interval; the schedule allows one in total.
func TestClaimRun(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: requires Docker")
	}
	ctx := t.Context()
	const interval = 2 * time.Second
	period := interval * 9 / 10 // as cmd/api claims it: slack for tick-to-claim latency
	replicaA, replicaB := mysql.NewRepository(testDB), mysql.NewRepository(testDB)
	name := "test-" + uuid.NewString()
	claim := func(r *mysql.Repository) bool {
		won, err := r.ClaimRun(ctx, name, period)
		require.NoError(t, err)
		return won
	}

	require.True(t, claim(replicaA), "first run ever is due")
	require.False(t, claim(replicaB), "B ticks within the same interval, out of phase with A")
	require.False(t, claim(replicaA))

	time.Sleep(interval) // A's own next tick; its previous claim landed a few ms after the tick
	require.True(t, claim(replicaA), "a tick exactly one interval later is due despite claim latency")
	require.False(t, claim(replicaB))
}

func newRepository(t *testing.T) *mysql.Repository {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: requires Docker")
	}
	_, err := testDB.ExecContext(t.Context(), "TRUNCATE TABLE calendar_entries")
	require.NoError(t, err)
	return mysql.NewRepository(testDB)
}

func insert(t *testing.T, repo *mysql.Repository, start, stop time.Time) calendar.Entry {
	t.Helper()
	e, err := calendar.NewEntry(uuid.Must(uuid.NewV7()), start, stop, now)
	require.NoError(t, err)
	require.NoError(t, repo.Create(t.Context(), e))
	return e
}
