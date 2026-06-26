package shipper

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

type fakeCloudWatchLogs struct {
	createErr error
	inputs    []*cloudwatchlogs.PutLogEventsInput
}

func (f *fakeCloudWatchLogs) CreateLogStream(context.Context, *cloudwatchlogs.CreateLogStreamInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	return &cloudwatchlogs.CreateLogStreamOutput{}, f.createErr
}

func (f *fakeCloudWatchLogs) PutLogEvents(_ context.Context, input *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
	f.inputs = append(f.inputs, input)
	return &cloudwatchlogs.PutLogEventsOutput{}, nil
}

func TestEnsureLogStreamAllowsAlreadyExists(t *testing.T) {
	shipper := NewWithClient(&fakeCloudWatchLogs{
		createErr: &cwltypes.ResourceAlreadyExistsException{},
	}, Config{LogGroupName: "group", LogStreamName: "stream"})

	if err := shipper.ensureLogStream(context.Background()); err != nil {
		t.Fatalf("ensureLogStream() error = %v", err)
	}
}

func TestEnsureLogStreamReturnsUnexpectedError(t *testing.T) {
	shipper := NewWithClient(&fakeCloudWatchLogs{
		createErr: errors.New("boom"),
	}, Config{LogGroupName: "group", LogStreamName: "stream"})

	if err := shipper.ensureLogStream(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

func TestEmitPrefixesAndTruncatesMessages(t *testing.T) {
	client := &fakeCloudWatchLogs{}
	shipper := NewWithClient(client, Config{LogGroupName: "group", LogStreamName: "stream"})

	shipper.Emit(context.Background(), "source", strings.Repeat("a", maxMessageBytes+1024))
	event := <-shipper.events
	message := aws.ToString(event.Message)

	if !strings.HasPrefix(message, "[source] ") {
		t.Fatalf("message prefix = %q", message[:20])
	}
	if !strings.Contains(message, "...[truncated]") {
		t.Fatal("expected truncation marker")
	}
}

func TestSendBatchSortsByTimestamp(t *testing.T) {
	client := &fakeCloudWatchLogs{}
	shipper := NewWithClient(client, Config{LogGroupName: "group", LogStreamName: "stream"})

	shipper.SendBatch(context.Background(), []cwltypes.InputLogEvent{
		{Timestamp: aws.Int64(20), Message: aws.String("second")},
		{Timestamp: aws.Int64(10), Message: aws.String("first")},
	})

	if len(client.inputs) != 1 {
		t.Fatalf("PutLogEvents calls = %d, want 1", len(client.inputs))
	}
	events := client.inputs[0].LogEvents
	if got := aws.ToString(events[0].Message); got != "first" {
		t.Fatalf("first event = %q", got)
	}
	if aws.ToString(client.inputs[0].LogGroupName) != "group" || aws.ToString(client.inputs[0].LogStreamName) != "stream" {
		t.Fatalf("unexpected destination: %+v", client.inputs[0])
	}
}
