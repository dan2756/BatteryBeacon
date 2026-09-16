//go:build !unix

package main

import (
	"os"
	"os/exec"
	"syscall"
)

// runHook runs cmd; it is killed if its context is done first.
func runHook(cmd *exec.Cmd) error {
	return cmd.Run()
}

// stopSignals are the signals that stop batt watch.
func stopSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
