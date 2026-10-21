// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// ownProcessGroup starts the child in its own process group, so a stop or a
// timeout reaches the program and everything it spawned (go test children),
// and nothing is left orphaned when the wrapper is told to stop.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// signalChild delivers sig to the child's whole process group, falling back
// to the child alone.
func signalChild(cmd *exec.Cmd, sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok && cmd.Process != nil {
		if syscall.Kill(-cmd.Process.Pid, s) == nil {
			return
		}
	}
	_ = cmd.Process.Signal(sig)
}

// watchdogScript kills process group $1 unless a line arrives on stdin. The
// wrapper holds the pipe's write end: it writes the line when Run finishes,
// and the kernel closes the pipe (EOF, no line) if the wrapper dies first,
// even by SIGKILL, which no Go code can intercept. Plain POSIX sh: dash has
// no "kill --", so the group is passed as a negative pid.
const watchdogScript = `read -r _ || kill -9 -"$1" 2>/dev/null`

// watchGroup starts the watchdog for the child's process group (its pgid is
// the child's pid, see ownProcessGroup). Setpgid takes the child out of the
// wrapper's group, so without this a SIGKILLed wrapper left the program and
// everything it spawned running. The watchdog sits in its own group, so a
// terminal signal to the wrapper does not take it down first. Call release
// once Wait returned; it tells the watchdog to exit without killing. Fail-open:
// if the watchdog cannot start, Run behaves as before.
func watchGroup(cmd *exec.Cmd) (release func()) {
	noop := func() {}
	if cmd.Process == nil {
		return noop
	}
	r, w, err := os.Pipe()
	if err != nil {
		return noop
	}
	wd := exec.Command("/bin/sh", "-c", watchdogScript, "downshift-watchdog", strconv.Itoa(cmd.Process.Pid))
	wd.Stdin = r
	wd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := wd.Start(); err != nil {
		_ = r.Close()
		_ = w.Close()
		return noop
	}
	_ = r.Close()
	return func() {
		_, _ = w.Write([]byte("\n"))
		_ = w.Close()
		_ = wd.Wait()
	}
}
