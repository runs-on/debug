# runs-on/debug

`runs-on/debug` starts a detached log shipper on a RunsOn Linux runner and writes host diagnostics to the instance CloudWatch Logs group.

Put it as early as possible in a job:

```yaml
steps:
  - uses: runs-on/debug@v1
  - uses: actions/checkout@v6
```

The action exits after starting the background process. Logs continue to ship if a later step hangs or the runner is cancelled.

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `stream_suffix` | `debug` | CloudWatch stream suffix. The final stream is `<instance-id>/<stream_suffix>`. |
| `snapshot_interval` | `5s` | Interval for periodic host snapshots. |
| `include_snapshots` | `true` | Whether to include periodic host snapshots. |

## CloudWatch Destination

The action writes to the existing RunsOn EC2 instance log group. It discovers configuration from:

- `RUNS_ON_LOG_GROUP_NAME`, falling back to `/etc/runs-on/bootstrap.env`
- `RUNS_ON_AWS_REGION` or `AWS_REGION`, falling back to `/etc/runs-on/bootstrap.env`
- `RUNS_ON_INSTANCE_ID`, falling back to IMDS

The default stream is `<instance-id>/debug`.

## Captured Logs

- `runs-on-bootstrap.service` journal
- systemd PID 1 journal
- kernel journal
- `/var/log/cloud-init-output.log`
- `/runs-on/output.log`
- `/var/log/syslog`, `/var/log/messages`, and `/var/log/kern.log` when present
- journal entries mentioning sudo, systemctl, shutdown, poweroff, reboot, or halt
- `/home/runner/_diag/*.log`
- periodic process, cgroup, memory, pressure, and `dmesg` snapshots

## Notes

- Linux `amd64` and `arm64` are supported.
- Windows prints a no-op warning for v1.
- Repeated invocations are idempotent through `/runs-on/debug-action.pid`.
- The runner role must allow CloudWatch Logs `CreateLogStream` and `PutLogEvents`.
