package sqs

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"

	"github.com/bashkirian/calendar-service/internal/calendar"
)

const maxBatchSize = 10 // SQS SendMessageBatch limit

// Connect reads the standard AWS env (AWS_REGION, AWS_ENDPOINT_URL, ...) and resolves the queue URL.
func Connect(ctx context.Context, queueName string) (*awssqs.Client, string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("load aws config: %w", err)
	}
	client := awssqs.NewFromConfig(cfg)
	out, err := client.GetQueueUrl(ctx, &awssqs.GetQueueUrlInput{QueueName: aws.String(queueName)})
	if err != nil {
		return nil, "", fmt.Errorf("resolve queue %q: %w", queueName, err)
	}
	return client, aws.ToString(out.QueueUrl), nil
}

var _ calendar.Publisher = (*Publisher)(nil)

type Publisher struct {
	client   *awssqs.Client
	queueURL string
}

func NewPublisher(client *awssqs.Client, queueURL string) *Publisher {
	return &Publisher{client: client, queueURL: queueURL}
}

// Publish stops at the first failing batch and returns the IDs SQS accepted, also from that batch.
func (p *Publisher) Publish(ctx context.Context, entries []calendar.Entry) ([]uuid.UUID, error) {
	sent := make([]uuid.UUID, 0, len(entries))
	for batch := range slices.Chunk(entries, maxBatchSize) {
		ok, err := p.send(ctx, batch)
		sent = append(sent, ok...)
		if err != nil {
			return sent, err
		}
	}
	return sent, nil
}

func (p *Publisher) send(ctx context.Context, batch []calendar.Entry) ([]uuid.UUID, error) {
	now := time.Now()
	reqs := make([]types.SendMessageBatchRequestEntry, len(batch))
	for i, e := range batch {
		body, err := encode(e, now)
		if err != nil {
			return nil, fmt.Errorf("encode entry %s: %w", e.ID, err)
		}
		reqs[i] = types.SendMessageBatchRequestEntry{
			Id:          aws.String(strconv.Itoa(i)), // index back into batch
			MessageBody: aws.String(body),
		}
	}
	out, err := p.client.SendMessageBatch(ctx, &awssqs.SendMessageBatchInput{QueueUrl: &p.queueURL, Entries: reqs})
	if err != nil {
		return nil, fmt.Errorf("send message batch: %w", err)
	}
	sent := make([]uuid.UUID, 0, len(out.Successful))
	for _, ok := range out.Successful {
		i, _ := strconv.Atoi(aws.ToString(ok.Id))
		sent = append(sent, batch[i].ID)
	}
	if len(out.Failed) > 0 {
		return sent, fmt.Errorf("send message batch: %d of %d failed, first: %s",
			len(out.Failed), len(batch), aws.ToString(out.Failed[0].Message))
	}
	return sent, nil
}
