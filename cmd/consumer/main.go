// Command consumer reads calendar entries from SQS and prints them to stdout.
package main

import (
	"cmp"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bashkirian/calendar-service/internal/calendar"
	"github.com/bashkirian/calendar-service/internal/sqs"
)

const visibility = 30 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("consumer stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Logs go to stderr so stdout carries only the printed entries.
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	client, queueURL, err := sqs.Connect(ctx, cmp.Or(os.Getenv("SQS_QUEUE_NAME"), "calendar-entries"))
	if err != nil {
		return err
	}
	log.Info("consumer started", "queue", queueURL)
	return sqs.NewConsumer(client, queueURL, visibility, printToStdout, log).Run(ctx)
}

func printToStdout(_ context.Context, e calendar.Entry) error {
	return printEntry(os.Stdout, e)
}
