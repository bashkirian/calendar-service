package sqs_test

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"

	"github.com/bashkirian/calendar-service/internal/calendar"
	"github.com/bashkirian/calendar-service/internal/sqs"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if testing.Short() {
		return m.Run()
	}
	ctx := context.Background()
	ctr, err := localstack.Run(ctx, "localstack/localstack:4.14")
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	if err != nil {
		log.Printf("start localstack: %v", err)
		return 1
	}
	endpoint, err := ctr.PortEndpoint(ctx, "4566/tcp", "http")
	if err != nil {
		log.Printf("localstack endpoint: %v", err)
		return 1
	}
	// Connect reads the standard AWS environment, exactly as in production.
	for k, v := range map[string]string{
		"AWS_ENDPOINT_URL":      endpoint,
		"AWS_REGION":            "eu-central-1",
		"AWS_ACCESS_KEY_ID":     "test",
		"AWS_SECRET_ACCESS_KEY": "test",
	} {
		_ = os.Setenv(k, v)
	}
	return m.Run()
}

func TestPublishAndConsume(t *testing.T) {
	client, queueURL := newQueue(t)
	entries := newEntries(t, 12) // more than one SendMessageBatch
	sent, err := sqs.NewPublisher(client, queueURL).Publish(t.Context(), entries)
	require.NoError(t, err)
	require.ElementsMatch(t, ids(entries), sent)

	var failures atomic.Int32
	received := make(chan calendar.Entry, len(entries))
	handle := func(_ context.Context, e calendar.Entry) error {
		if e.ID == entries[0].ID && failures.Add(1) == 1 {
			return errors.New("transient failure")
		}
		received <- e
		return nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error)
	go func() {
		done <- sqs.NewConsumer(client, queueURL, time.Second, handle, slog.New(slog.DiscardHandler)).Run(ctx)
	}()

	var got []calendar.Entry
	timeout := time.After(30 * time.Second)
	for len(got) < len(entries) {
		select {
		case e := <-received:
			got = append(got, e)
		case <-timeout:
			t.Fatalf("received %d of %d entries", len(got), len(entries))
		}
	}
	cancel()
	require.NoError(t, <-done, "consumer stops cleanly on cancellation")

	require.ElementsMatch(t, entries, got, "every entry arrives once handled successfully, the failed one via redelivery")
	require.EqualValues(t, 2, failures.Load(), "the failed message was redelivered")
	requireQueueEmpty(t, client, queueURL)
}

func newQueue(t *testing.T) (*awssqs.Client, string) {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: requires Docker")
	}
	cfg, err := config.LoadDefaultConfig(t.Context())
	require.NoError(t, err)
	name := "test-" + uuid.NewString()
	_, err = awssqs.NewFromConfig(cfg).CreateQueue(t.Context(), &awssqs.CreateQueueInput{
		QueueName: aws.String(name),
	})
	require.NoError(t, err)

	client, queueURL, err := sqs.Connect(t.Context(), name)
	require.NoError(t, err)
	return client, queueURL
}

func newEntries(t *testing.T, n int) []calendar.Entry {
	t.Helper()
	now := time.Now()
	entries := make([]calendar.Entry, n)
	for i := range entries {
		e, err := calendar.NewEntry(uuid.Must(uuid.NewV7()), now, now.Add(time.Duration(i)*time.Hour), now)
		require.NoError(t, err)
		entries[i] = e
	}
	return entries
}

func requireQueueEmpty(t *testing.T, client *awssqs.Client, queueURL string) {
	t.Helper()
	out, err := client.GetQueueAttributes(t.Context(), &awssqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameApproximateNumberOfMessages,
			types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		},
	})
	require.NoError(t, err)
	for name, v := range out.Attributes {
		require.Equal(t, "0", v, name)
	}
}

func ids(entries []calendar.Entry) []uuid.UUID {
	out := make([]uuid.UUID, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}
