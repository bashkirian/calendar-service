#!/bin/sh
# Runs when LocalStack is ready: main queue with a dead-letter queue behind it.
set -eu

QUEUE=calendar-entries
DLQ="$QUEUE-dlq"

awslocal sqs create-queue --queue-name "$DLQ" >/dev/null
DLQ_ARN=$(awslocal sqs get-queue-attributes \
  --queue-url "$(awslocal sqs get-queue-url --queue-name "$DLQ" --query QueueUrl --output text)" \
  --attribute-names QueueArn --query Attributes.QueueArn --output text)

# Visibility timeout is set by the consumer per receive, next to the code that depends on it.
awslocal sqs create-queue --queue-name "$QUEUE" --attributes "{
  \"RedrivePolicy\": \"{\\\"deadLetterTargetArn\\\":\\\"$DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"5\\\"}\"
}" >/dev/null

echo "SQS queues ready: $QUEUE -> $DLQ"
