//go:build unix

package main

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// hookGrace is how long a running hook gets to exit after SIGTERM when batt
// is stopped, before whatever is left of it gets SIGKILL.
const hookGrace = 5 * time.Second

// runHook runs cmd in a process group of its own. When cmd's context is done
// before it finishes, the whole group gets SIGTERM, not just /bin/sh, and
// runHook waits up to hookGrace for it to exit before killing the rest, so
// nothing the hook started outlives batt.
func runHook(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid
		syscall.Kill(-pgid, syscall.SIGTERM)
		for end := time.Now().Add(hookGrace); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
			if syscall.Kill(-pgid, 0) == syscall.ESRCH {
				return nil // all gone
			}
		}
		return syscall.Kill(-pgid, syscall.SIGKILL)
	}
	return cmd.Run()
}

// stopSignals are the signals that stop batt watch. Since hooks run in their
// own process group, a closing terminal's SIGHUP doesn't reach them, so batt
// stops on SIGHUP too (unless it runs under nohup) and stops the hook.
func stopSignals() []os.Signal {
	sigs := []os.Signal{os.Interrupt, syscall.SIGTERM}
	if !signal.Ignored(syscall.SIGHUP) {
		sigs = append(sigs, syscall.SIGHUP)
	}
	return sigs
}
