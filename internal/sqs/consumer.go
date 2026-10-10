package sqs

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/bashkirian/calendar-service/internal/calendar"
)

const (
	longPollSeconds = 20
	retryDelay      = time.Second
)

// Handler must be idempotent: SQS delivers at least once.
type Handler func(ctx context.Context, e calendar.Entry) error

type Consumer struct {
	client     *awssqs.Client
	queueURL   string
	visibility time.Duration
	handle     Handler
	log        *slog.Logger
}

// NewConsumer: the whole received batch must be handled within visibility.
func NewConsumer(client *awssqs.Client, queueURL string, visibility time.Duration, handle Handler, log *slog.Logger) *Consumer {
	return &Consumer{client: client, queueURL: queueURL, visibility: visibility, handle: handle, log: log}
}

// Run deletes only handled messages; failed ones reappear after visibility and end in the DLQ.
func (c *Consumer) Run(ctx context.Context) error {
	for {
		out, err := c.client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
			QueueUrl:            &c.queueURL,
			MaxNumberOfMessages: 10,
			WaitTimeSeconds:     longPollSeconds,
			VisibilityTimeout:   int32(c.visibility / time.Second), //nolint:gosec // seconds, SQS caps it at 12h
		})
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			c.log.ErrorContext(ctx, "receive messages", "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryDelay):
			}
			continue
		}
		if len(out.Messages) > 0 {
			// Finish a received batch on shutdown instead of abandoning it mid-way.
			c.processBatch(context.WithoutCancel(ctx), out.Messages)
		}
	}
}

// processBatch uses one deadline: visibility started for all messages at receive time.
func (c *Consumer) processBatch(ctx context.Context, msgs []types.Message) {
	ctx, cancel := context.WithTimeout(ctx, c.visibility*5/6) // leave time to delete
	defer cancel()

	done := make([]types.DeleteMessageBatchRequestEntry, 0, len(msgs))
	for _, m := range msgs {
		e, err := decode(aws.ToString(m.Body))
		if err == nil {
			err = c.handle(ctx, e)
		}
		if err != nil {
			c.log.ErrorContext(ctx, "handle message, leaving it for retry", "message_id", aws.ToString(m.MessageId), "err", err)
			continue
		}
		done = append(done, types.DeleteMessageBatchRequestEntry{Id: m.MessageId, ReceiptHandle: m.ReceiptHandle})
	}
	if len(done) == 0 {
		return
	}

	out, err := c.client.DeleteMessageBatch(ctx, &awssqs.DeleteMessageBatchInput{QueueUrl: &c.queueURL, Entries: done})
	if err != nil {
		c.log.ErrorContext(ctx, "delete messages", "err", err)
		return
	}
	for _, f := range out.Failed {
		c.log.ErrorContext(ctx, "delete message", "message_id", aws.ToString(f.Id), "err", aws.ToString(f.Message))
	}
}
