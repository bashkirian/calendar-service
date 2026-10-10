-- name: CreateEntry :exec
INSERT INTO calendar_entries (id, start_date, stop_date, created_at, updated_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListActive :many
SELECT * FROM calendar_entries WHERE stop_date >= ? ORDER BY stop_date, id;

-- name: ListUnsentActive :many
SELECT * FROM calendar_entries
WHERE sent_at IS NULL AND stop_date >= ?
ORDER BY stop_date, id
LIMIT ?;

-- name: MarkSent :exec
UPDATE calendar_entries SET sent_at = ? WHERE id IN (sqlc.slice(ids));

-- Affected rows: 1 = first run, 2 = due, 0 = not due. DB clock (session in UTC), so replica clock skew is irrelevant.
-- name: ClaimJobRun :execrows
INSERT INTO scheduled_jobs (name, next_run_at)
VALUES (sqlc.arg(name), NOW(3) + INTERVAL sqlc.arg(interval_us) MICROSECOND)
ON DUPLICATE KEY UPDATE
    next_run_at = IF(next_run_at <= NOW(3), NOW(3) + INTERVAL sqlc.arg(interval_us) MICROSECOND, next_run_at);
