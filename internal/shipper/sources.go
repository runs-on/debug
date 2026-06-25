package shipper

type StreamSource struct {
	Name    string
	Command string
	Args    []string
}

func StreamSources() []StreamSource {
	return []StreamSource{
		{
			Name:    "journal:runs-on-bootstrap",
			Command: "journalctl",
			Args:    []string{"-f", "-n", "300", "-o", "short-iso", "--no-pager", "-u", "runs-on-bootstrap.service"},
		},
		{
			Name:    "journal:pid1",
			Command: "journalctl",
			Args:    []string{"-f", "-n", "300", "-o", "short-iso", "--no-pager", "_PID=1"},
		},
		{
			Name:    "journal:kernel",
			Command: "journalctl",
			Args:    []string{"-k", "-f", "-n", "300", "-o", "short-iso", "--no-pager"},
		},
		{
			Name:    "file:cloud-init-output",
			Command: "bash",
			Args:    []string{"-lc", `if [ -e /var/log/cloud-init-output.log ]; then tail -n +1 -F /var/log/cloud-init-output.log; else echo "missing /var/log/cloud-init-output.log"; sleep infinity; fi`},
		},
		{
			Name:    "file:runs-on-output",
			Command: "bash",
			Args:    []string{"-lc", `if [ -e /runs-on/output.log ]; then tail -n +1 -F /runs-on/output.log; else echo "missing /runs-on/output.log"; sleep infinity; fi`},
		},
		{
			Name:    "file:syslog",
			Command: "bash",
			Args:    []string{"-lc", `for f in /var/log/syslog /var/log/messages; do [ -e "$f" ] && exec tail -n +1 -F "$f"; done; echo "missing syslog/messages"; sleep infinity`},
		},
		{
			Name:    "file:kern.log",
			Command: "bash",
			Args:    []string{"-lc", `if [ -e /var/log/kern.log ]; then tail -n +1 -F /var/log/kern.log; else echo "missing /var/log/kern.log"; sleep infinity; fi`},
		},
		{
			Name:    "journal:shutdown-commands",
			Command: "bash",
			Args:    []string{"-lc", `journalctl -f -n 300 -o short-iso --no-pager | grep --line-buffered -Ei 'sudo|systemctl|shutdown|poweroff|reboot|halt'`},
		},
	}
}

func InitialSnapshotCommand() string {
	return `
set +e
echo "===== runs-on-bootstrap.service ====="
systemctl show runs-on-bootstrap.service
echo "===== filtered ps ====="
ps -eo pid,ppid,stat,etime,pcpu,pmem,args --sort=-pcpu | head -80
echo "===== systemd-cgls ====="
systemd-cgls --no-pager
echo "===== free -m ====="
free -m
echo "===== pressure ====="
for f in /proc/pressure/*; do [ -e "$f" ] && echo "--- $f ---" && cat "$f"; done
echo "===== recent dmesg ====="
dmesg -T | tail -300
`
}

func PeriodicSnapshotCommand() string {
	return `
set +e
echo "===== snapshot $(date -u --iso-8601=seconds) ====="
systemctl show runs-on-bootstrap.service -p ActiveState -p SubState -p MainPID -p ExecMainPID -p NRestarts -p Result
echo "===== filtered ps ====="
ps -eo pid,ppid,stat,etime,pcpu,pmem,args --sort=-pcpu | head -50
echo "===== systemd-cgls ====="
systemd-cgls --no-pager | head -200
echo "===== free -m ====="
free -m
echo "===== pressure ====="
for f in /proc/pressure/*; do [ -e "$f" ] && echo "--- $f ---" && cat "$f"; done
echo "===== recent dmesg ====="
dmesg -T | tail -120
`
}
