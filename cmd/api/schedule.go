package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/bashkirian/calendar-service/internal/mysql"
)

// every runs on start, then each interval; calls never overlap.
func every(ctx context.Context, interval time.Duration, run func(context.Context)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		run(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// guarded runs job only on the replica that wins this interval, and stops it before the next one.
func guarded(repo *mysql.Repository, name string, interval time.Duration, job func(context.Context) (int, error), log *slog.Logger) func(context.Context) {
	// 10% slack: the claim lands ms after the tick, and the same replica's next tick must still win.
	due := interval - interval/10
	log = log.With("job", name)
	return func(ctx context.Context) {
		won, err := repo.ClaimRun(ctx, name, due)
		if err != nil {
			log.ErrorContext(ctx, "claim run", "err", err)
			return
		}
		if !won {
			log.InfoContext(ctx, "run skipped: another replica ran it this interval")
			return
		}
		ctx, cancel := context.WithTimeout(ctx, due)
		defer cancel()
		n, err := job(ctx)
		if err != nil {
			log.ErrorContext(ctx, "run failed", "err", err, "processed", n)
			return
		}
		log.InfoContext(ctx, "run done", "processed", n)
	}
}
