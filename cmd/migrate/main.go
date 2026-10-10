// Command migrate applies database migrations; run it as a deploy step before the api.
package main

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bashkirian/calendar-service/internal/mysql"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := mysql.Open(ctx, cmp.Or(os.Getenv("MYSQL_DSN"), "calendar:calendar@tcp(localhost:3306)/calendar"))
	if err != nil {
		return err
	}
	defer db.Close()
	return mysql.Migrate(ctx, db)
}
