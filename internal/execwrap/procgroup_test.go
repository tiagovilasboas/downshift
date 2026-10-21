// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A stop reaches the whole process group: the grandchild a test binary would
// leave behind dies with the signal.
func TestRun_SignalReachesGrandchildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	fakeBin(t, "go", "sleep 30 & echo $! > "+pidFile+"; wait")
	sig := make(chan os.Signal, 1)
	go func() {
		for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
			if _, err := os.Stat(pidFile); err == nil {
				time.Sleep(100 * time.Millisecond) // let the pid be written
				break
			}
		}
		sig <- syscall.SIGTERM
	}()
	var out, errb bytes.Buffer
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, Signals: sig})
	if r.ExitClass != ExitClassSignal {
		t.Fatalf("result = %+v", r)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatal("grandchild survived the stop")
}

// TestHelperKilledWrapper is the wrapper process for
// TestRun_WrapperKilledLeavesNoChild; it does nothing in a normal run.
func TestHelperKilledWrapper(t *testing.T) {
	if os.Getenv("EXECWRAP_KILLED_WRAPPER") != "1" {
		t.Skip("helper process only")
	}
	Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, Signals: make(chan os.Signal)})
}

// The wrapper itself is SIGKILLed, so it never gets to forward a signal: the
// program and its grandchild must still die, not run on in their own group.
func TestRun_WrapperKilledLeavesNoChild(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pids")
	fakeBin(t, "go", "sleep 30 & echo \"$$ $!\" > "+pidFile+".tmp && mv "+pidFile+".tmp "+pidFile+"; wait")
	wrapper := exec.Command(os.Args[0], "-test.run=^TestHelperKilledWrapper$")
	wrapper.Env = append(os.Environ(), "EXECWRAP_KILLED_WRAPPER=1")
	if err := wrapper.Start(); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if b, err := os.ReadFile(pidFile); err == nil {
			raw = b
			break
		}
	}
	if raw == nil {
		_ = wrapper.Process.Kill()
		t.Fatal("wrapped program never started")
	}
	var pids []int
	for _, f := range strings.Fields(string(raw)) {
		pid, err := strconv.Atoi(f)
		if err != nil {
			t.Fatal(err)
		}
		pids = append(pids, pid)
	}
	t.Cleanup(func() {
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	_ = wrapper.Process.Kill()
	_ = wrapper.Wait()
	deadline := time.Now().Add(5 * time.Second)
	for _, pid := range pids {
		for processAlive(pid) {
			if time.Now().After(deadline) {
				t.Fatalf("pid %d (of program %d, grandchild %d) survived the wrapper's SIGKILL", pid, pids[0], pids[1])
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// processAlive reports whether pid exists and is not a zombie (a killed
// orphan stays a zombie until its new parent reaps it, which a minimal
// container init may never do).
func processAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true // no procfs (macOS): launchd reaps orphans
	}
	// Format: pid (comm) state ...; comm may contain spaces.
	s := string(stat)
	if i := strings.LastIndexByte(s, ')'); i >= 0 && i+2 < len(s) {
		return s[i+2] != 'Z'
	}
	return true
}
