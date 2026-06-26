package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

type fakeIMDS struct {
	doc *imds.GetInstanceIdentityDocumentOutput
}

func (f fakeIMDS) GetInstanceIdentityDocument(context.Context, *imds.GetInstanceIdentityDocumentInput, ...func(*imds.Options)) (*imds.GetInstanceIdentityDocumentOutput, error) {
	return f.doc, nil
}

func TestDiscoverFromEnvironment(t *testing.T) {
	t.Setenv("RUNS_ON_LOG_GROUP_NAME", "group")
	t.Setenv("RUNS_ON_AWS_REGION", "us-east-1")
	t.Setenv("RUNS_ON_INSTANCE_ID", "i-123")
	t.Setenv("INPUT_STREAM_SUFFIX", "/deep/debug/")
	t.Setenv("INPUT_SNAPSHOT_INTERVAL", "2s")
	t.Setenv("INPUT_INCLUDE_SNAPSHOTS", "false")
	t.Setenv("RUNS_ON_DEBUG_STATE_DIR", t.TempDir())

	runtime, err := Discover(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if runtime.LogGroupName != "group" || runtime.Region != "us-east-1" || runtime.InstanceID != "i-123" {
		t.Fatalf("unexpected runtime: %+v", runtime)
	}
	if runtime.StreamName() != "i-123/deep/debug" {
		t.Fatalf("StreamName() = %q", runtime.StreamName())
	}
	if runtime.IncludeSnapshots {
		t.Fatal("expected IncludeSnapshots false")
	}
}

func TestDiscoverFallsBackToBootstrapEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bootstrap.env")
	if err := os.WriteFile(path, []byte(`RUNS_ON_LOG_GROUP_NAME="group"
AWS_REGION='eu-west-1'
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("RUNS_ON_INSTANCE_ID", "i-456")

	runtime, err := Discover(context.Background(), path, nil)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if runtime.LogGroupName != "group" || runtime.Region != "eu-west-1" || runtime.InstanceID != "i-456" {
		t.Fatalf("unexpected runtime: %+v", runtime)
	}
}

func TestDiscoverFallsBackToIMDSForRegionAndInstance(t *testing.T) {
	t.Setenv("RUNS_ON_LOG_GROUP_NAME", "group")

	runtime, err := Discover(context.Background(), "", fakeIMDS{doc: &imds.GetInstanceIdentityDocumentOutput{
		InstanceIdentityDocument: imds.InstanceIdentityDocument{
			InstanceID: "i-imds",
			Region:     "ap-south-1",
		},
	}})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if runtime.Region != "ap-south-1" || runtime.InstanceID != "i-imds" {
		t.Fatalf("unexpected runtime: %+v", runtime)
	}
}
