package shipper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

const (
	flushInterval      = time.Second
	maxBatchEvents     = 200
	maxBatchBytes      = 800 * 1024
	maxMessageBytes    = 240 * 1024
	scannerMaxCapacity = 1024 * 1024
)

type CloudWatchLogsAPI interface {
	CreateLogStream(context.Context, *cloudwatchlogs.CreateLogStreamInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	PutLogEvents(context.Context, *cloudwatchlogs.PutLogEventsInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
}

type Config struct {
	LogGroupName     string
	LogStreamName    string
	Region           string
	SnapshotInterval time.Duration
	IncludeSnapshots bool
}

type Shipper struct {
	cwl    CloudWatchLogsAPI
	config Config
	events chan cwltypes.InputLogEvent
}

func New(ctx context.Context, cfg Config) (*Shipper, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return NewWithClient(cloudwatchlogs.NewFromConfig(awsCfg), cfg), nil
}

func NewWithClient(cwl CloudWatchLogsAPI, cfg Config) *Shipper {
	return &Shipper{
		cwl:    cwl,
		config: cfg,
		events: make(chan cwltypes.InputLogEvent, 4096),
	}
}

func (s *Shipper) Run(ctx context.Context) error {
	if err := s.ensureLogStream(ctx); err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.flushLoop(ctx)
	}()

	s.Emit(ctx, "debug", fmt.Sprintf("===== RUNSON DEBUG LOG SHIPPING START %s =====", time.Now().UTC().Format(time.RFC3339)))
	s.runCommandSnapshot(ctx, "initial", InitialSnapshotCommand())

	for _, source := range StreamSources() {
		source := source
		go s.streamCommand(ctx, source.Name, source.Command, source.Args...)
	}
	go s.watchRunnerDiagLogs(ctx)
	if s.config.IncludeSnapshots {
		go s.snapshotLoop(ctx)
	}

	<-ctx.Done()
	<-done
	return ctx.Err()
}

func (s *Shipper) ensureLogStream(ctx context.Context) error {
	_, err := s.cwl.CreateLogStream(ctx, &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(s.config.LogGroupName),
		LogStreamName: aws.String(s.config.LogStreamName),
	})
	if err == nil {
		return nil
	}
	var exists *cwltypes.ResourceAlreadyExistsException
	if errors.As(err, &exists) {
		return nil
	}
	return fmt.Errorf("create log stream %s/%s: %w", s.config.LogGroupName, s.config.LogStreamName, err)
}

func (s *Shipper) Emit(ctx context.Context, source string, message string) {
	message = strings.TrimRight(message, "\r\n")
	if message == "" {
		return
	}
	if len(message) > maxMessageBytes {
		message = message[:maxMessageBytes] + " ...[truncated]"
	}
	if source != "" {
		message = "[" + source + "] " + message
	}

	event := cwltypes.InputLogEvent{
		Message:   aws.String(message),
		Timestamp: aws.Int64(time.Now().UnixMilli()),
	}
	select {
	case s.events <- event:
	case <-ctx.Done():
	}
}

func (s *Shipper) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]cwltypes.InputLogEvent, 0, maxBatchEvents)
	batchBytes := 0
	flush := func() {
		if len(batch) == 0 {
			return
		}
		s.SendBatch(ctx, batch)
		batch = make([]cwltypes.InputLogEvent, 0, maxBatchEvents)
		batchBytes = 0
	}

	for {
		select {
		case event := <-s.events:
			eventBytes := len(aws.ToString(event.Message)) + 26
			if len(batch) >= maxBatchEvents || batchBytes+eventBytes > maxBatchBytes {
				flush()
			}
			batch = append(batch, event)
			batchBytes += eventBytes
		case <-ticker.C:
			flush()
		case <-ctx.Done():
			flush()
			return
		}
	}
}

func (s *Shipper) SendBatch(ctx context.Context, events []cwltypes.InputLogEvent) {
	if len(events) == 0 {
		return
	}
	sort.SliceStable(events, func(i, j int) bool {
		return aws.ToInt64(events[i].Timestamp) < aws.ToInt64(events[j].Timestamp)
	})
	_, _ = s.cwl.PutLogEvents(ctx, &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(s.config.LogGroupName),
		LogStreamName: aws.String(s.config.LogStreamName),
		LogEvents:     events,
	})
}

func (s *Shipper) streamCommand(ctx context.Context, name string, command string, args ...string) {
	cmd := exec.CommandContext(ctx, command, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.Emit(ctx, name, fmt.Sprintf("stdout pipe error: %v", err))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.Emit(ctx, name, fmt.Sprintf("stderr pipe error: %v", err))
		return
	}
	if err := cmd.Start(); err != nil {
		s.Emit(ctx, name, fmt.Sprintf("start error: %v", err))
		return
	}

	done := make(chan struct{}, 2)
	go s.scanLines(ctx, name, stdout, done)
	go s.scanLines(ctx, name, stderr, done)
	<-done
	<-done

	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		s.Emit(ctx, name, fmt.Sprintf("command exited: %v", err))
	}
}

func (s *Shipper) scanLines(ctx context.Context, name string, reader io.Reader, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), scannerMaxCapacity)
	for scanner.Scan() {
		s.Emit(ctx, name, scanner.Text())
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		s.Emit(ctx, name, fmt.Sprintf("scan error: %v", err))
	}
}

func (s *Shipper) watchRunnerDiagLogs(ctx context.Context) {
	s.streamCommand(ctx, "runner-diag-watch", "bash", "-lc", `
seen=""
while true; do
  for f in /home/runner/_diag/*.log; do
    [ -e "$f" ] || continue
    case " $seen " in
      *" $f "*) ;;
      *)
        seen="$seen $f"
        tail -n +1 -F "$f" 2>&1 | sed -u "s#^#[runner-diag:$f] #" &
        ;;
    esac
  done
  sleep 1
done
`)
}

func (s *Shipper) snapshotLoop(ctx context.Context) {
	interval := s.config.SnapshotInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.runCommandSnapshot(ctx, "snapshot", PeriodicSnapshotCommand())
		case <-ctx.Done():
			return
		}
	}
}

func (s *Shipper) runCommandSnapshot(ctx context.Context, source string, script string) {
	snapshotCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(snapshotCtx, "bash", "-lc", script)
	out, err := cmd.CombinedOutput()
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		s.Emit(ctx, source, line)
	}
	if err != nil && ctx.Err() == nil {
		s.Emit(ctx, source, fmt.Sprintf("snapshot error: %v", err))
	}
}
