package config

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

const DefaultBootstrapEnvPath = "/etc/runs-on/bootstrap.env"

type IMDSClient interface {
	GetInstanceIdentityDocument(context.Context, *imds.GetInstanceIdentityDocumentInput, ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error)
}

type Runtime struct {
	LogGroupName     string
	Region           string
	InstanceID       string
	StreamSuffix     string
	SnapshotInterval time.Duration
	IncludeSnapshots bool
	StateDir         string
}

func Discover(ctx context.Context, bootstrapEnvPath string, imdsClient IMDSClient) (Runtime, error) {
	bootstrapEnv, _ := readBootstrapEnv(bootstrapEnvPath)

	runtime := Runtime{
		LogGroupName:     firstNonEmpty(os.Getenv("RUNS_ON_LOG_GROUP_NAME"), bootstrapEnv["RUNS_ON_LOG_GROUP_NAME"]),
		Region:           firstNonEmpty(os.Getenv("RUNS_ON_AWS_REGION"), os.Getenv("AWS_REGION"), bootstrapEnv["AWS_REGION"]),
		InstanceID:       os.Getenv("RUNS_ON_INSTANCE_ID"),
		StreamSuffix:     normalizeStreamSuffix(firstNonEmpty(os.Getenv("INPUT_STREAM_SUFFIX"), "debug")),
		SnapshotInterval: parseDurationOrDefault(os.Getenv("INPUT_SNAPSHOT_INTERVAL"), 5*time.Second),
		IncludeSnapshots: parseBoolOrDefault(os.Getenv("INPUT_INCLUDE_SNAPSHOTS"), true),
		StateDir:         firstNonEmpty(os.Getenv("RUNS_ON_DEBUG_STATE_DIR"), "/runs-on"),
	}

	if runtime.InstanceID == "" || runtime.Region == "" {
		if imdsClient != nil {
			doc, err := imdsClient.GetInstanceIdentityDocument(ctx, &imds.GetInstanceIdentityDocumentInput{})
			if err == nil && doc != nil {
				if runtime.InstanceID == "" {
					runtime.InstanceID = doc.InstanceID
				}
				if runtime.Region == "" {
					runtime.Region = doc.Region
				}
			}
		}
	}

	var missing []string
	if runtime.LogGroupName == "" {
		missing = append(missing, "RUNS_ON_LOG_GROUP_NAME")
	}
	if runtime.Region == "" {
		missing = append(missing, "RUNS_ON_AWS_REGION or AWS_REGION")
	}
	if runtime.InstanceID == "" {
		missing = append(missing, "RUNS_ON_INSTANCE_ID")
	}
	if len(missing) > 0 {
		return Runtime{}, fmt.Errorf("missing runtime config: %s", strings.Join(missing, ", "))
	}

	return runtime, nil
}

func (r Runtime) StreamName() string {
	return r.InstanceID + "/" + r.StreamSuffix
}

func readBootstrapEnv(path string) (map[string]string, error) {
	out := map[string]string{}
	if path == "" {
		return out, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		out[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return out, scanner.Err()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeStreamSuffix(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" {
		return "debug"
	}
	return value
}

func parseDurationOrDefault(value string, fallback time.Duration) time.Duration {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseBoolOrDefault(value string, fallback bool) bool {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
