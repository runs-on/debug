package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/runs-on/debug/internal/config"
	"github.com/runs-on/debug/internal/daemon"
	"github.com/runs-on/debug/internal/shipper"
)

func main() {
	if runtime.GOOS == "windows" {
		fmt.Println("RunsOn debug log shipping is a no-op on Windows for v1.")
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "ship" {
		if err := runShipper(os.Args[2:]); err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "runs-on/debug shipper failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := startDaemon(); err != nil {
		fmt.Fprintf(os.Stderr, "runs-on/debug failed: %v\n", err)
		os.Exit(1)
	}
}

func startDaemon() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runtimeConfig, err := config.Discover(ctx, config.DefaultBootstrapEnvPath, imds.New(imds.Options{}))
	if err != nil {
		return err
	}

	paths := daemon.Paths{StateDir: runtimeConfig.StateDir}
	if err := daemon.EnsureStateDir(runtimeConfig.StateDir); err != nil {
		return fmt.Errorf("prepare state dir %s: %w", runtimeConfig.StateDir, err)
	}

	if pid, ok := daemon.RunningPID(paths.PIDFile()); ok {
		fmt.Println(daemon.AlreadyRunningMessage(pid, runtimeConfig.StreamName()))
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	args := []string{
		"ship",
		"--log-group", runtimeConfig.LogGroupName,
		"--region", runtimeConfig.Region,
		"--instance-id", runtimeConfig.InstanceID,
		"--stream-suffix", runtimeConfig.StreamSuffix,
		"--snapshot-interval", runtimeConfig.SnapshotInterval.String(),
		"--include-snapshots", strconv.FormatBool(runtimeConfig.IncludeSnapshots),
	}
	pid, err := daemon.StartDetached(executable, args, paths.LogFile())
	if err != nil {
		return fmt.Errorf("start detached shipper: %w", err)
	}
	if err := daemon.WritePID(paths.PIDFile(), pid); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	fmt.Printf("RunsOn debug log shipping started with PID %d\n", pid)
	fmt.Printf("CloudWatch stream: %s\n", runtimeConfig.StreamName())
	return nil
}

func runShipper(args []string) error {
	flags := flag.NewFlagSet("ship", flag.ContinueOnError)
	logGroupName := flags.String("log-group", "", "CloudWatch Logs group name")
	region := flags.String("region", "", "AWS region")
	instanceID := flags.String("instance-id", "", "EC2 instance id")
	streamSuffix := flags.String("stream-suffix", "debug", "CloudWatch stream suffix")
	snapshotInterval := flags.Duration("snapshot-interval", 5*time.Second, "snapshot interval")
	includeSnapshots := flags.Bool("include-snapshots", true, "include periodic snapshots")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *logGroupName == "" || *region == "" || *instanceID == "" {
		return fmt.Errorf("missing required shipper config")
	}

	ctx, stop := terminationContext()
	defer stop()

	ship, err := shipper.New(ctx, shipper.Config{
		LogGroupName:     *logGroupName,
		LogStreamName:    *instanceID + "/" + *streamSuffix,
		Region:           *region,
		SnapshotInterval: *snapshotInterval,
		IncludeSnapshots: *includeSnapshots,
	})
	if err != nil {
		return err
	}
	return ship.Run(ctx)
}
