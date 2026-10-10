// Command api serves the HTTP API and publishes active entries on a schedule.
package main

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bashkirian/calendar-service/internal/calendar"
	"github.com/bashkirian/calendar-service/internal/httpapi"
	"github.com/bashkirian/calendar-service/internal/mysql"
	"github.com/bashkirian/calendar-service/internal/sqs"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	interval, err := time.ParseDuration(cmp.Or(os.Getenv("PUBLISH_INTERVAL"), "5m")) // 0 disables scheduling
	if err != nil {
		return err
	}
	once, err := strconv.ParseBool(cmp.Or(os.Getenv("PUBLISH_ONCE"), "false"))
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := mysql.Open(ctx, cmp.Or(os.Getenv("MYSQL_DSN"), "calendar:calendar@tcp(localhost:3306)/calendar"))
	if err != nil {
		return err
	}
	defer db.Close()

	client, queueURL, err := sqs.Connect(ctx, cmp.Or(os.Getenv("SQS_QUEUE_NAME"), "calendar-entries"))
	if err != nil {
		return err
	}
	repo := mysql.NewRepository(db)
	svc := calendar.NewService(repo, sqs.NewPublisher(client, queueURL), time.Now)
	srv := &http.Server{
		Addr:              cmp.Or(os.Getenv("HTTP_ADDR"), ":8080"),
		Handler:           httpapi.NewRouter(svc, db.PingContext, log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})
	if interval > 0 {
		name, job := "publish-active", svc.PublishActive
		if once {
			name, job = "publish-unsent", svc.PublishUnsent
		}
		g.Go(func() error {
			every(ctx, interval, guarded(repo, name, interval, job, log))
			return nil
		})
	}
	return g.Wait()
}
