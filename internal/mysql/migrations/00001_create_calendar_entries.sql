-- +goose Up
CREATE TABLE calendar_entries (
    id         BINARY(16)  NOT NULL,
    start_date DATETIME(3) NOT NULL,
    stop_date  DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL,
    sent_at    DATETIME(3) NULL, -- set once the entry is published in once-only mode
    PRIMARY KEY (id),
    INDEX idx_calendar_entries_stop_date (stop_date, id),
    INDEX idx_calendar_entries_unsent (sent_at, stop_date),
    CONSTRAINT chk_calendar_entries_period CHECK (stop_date >= start_date)
);

-- Schedule shared by all replicas (see ClaimJobRun).
CREATE TABLE scheduled_jobs (
    name        VARCHAR(64) NOT NULL,
    next_run_at DATETIME(3) NOT NULL,
    PRIMARY KEY (name)
);

-- +goose Down
DROP TABLE scheduled_jobs;
DROP TABLE calendar_entries;
