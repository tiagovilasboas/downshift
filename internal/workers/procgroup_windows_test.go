// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package workers

import (
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// TestKillGroupStopsTheWholeTree starts a shell that spawns a long child and
// proves killGroup ends the shell and releases its job.
func TestKillGroupStopsTheWholeTree(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping -n 60 127.0.0.1 >NUL")
	ownProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	adoptProcess(cmd)
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	time.Sleep(500 * time.Millisecond)
	if err := killGroup(cmd); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("worker still running after killGroup")
	}
	if _, ok := jobs.Load(cmd); ok {
		t.Fatal("job not released")
	}
}

var procIsProcessInJob = kernel32.NewProc("IsProcessInJob")

// TestWorkerChildIsBornInTheJob starts a worker shell that spawns a long child
// at once and proves the child is in the worker's job and ends when the job is
// closed. The sleep between Start and adoptProcess widens the window the old
// code left open: the worker ran before it joined the job, so a child spawned
// then escaped it and closing the job left the child running. Started
// suspended, the worker runs nothing until it is in the job, so the outcome
// does not depend on the sleep.
func TestWorkerChildIsBornInTheJob(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping -n 60 127.0.0.1 >NUL")
	ownProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		releaseProcess(cmd)
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		select {
		case <-done:
			return // never taskkill a pid that may have been reused
		default:
		}
		_ = killGroup(cmd)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	})
	time.Sleep(time.Second)
	adoptProcess(cmd)
	v, ok := jobs.Load(cmd)
	if !ok || !v.(*workerJob).assigned {
		t.Fatal("the worker was not assigned to its job")
	}
	child := waitForChild(t, uint32(cmd.Process.Pid), "ping.exe", 10*time.Second)
	if !inJob(t, child, v.(*workerJob).handle) {
		t.Fatal("the worker's child is not in the worker's job")
	}
	releaseProcess(cmd) // closing a kill-on-close job ends its tree, no taskkill
	if ev, _ := syscall.WaitForSingleObject(child, 10_000); ev != syscall.WAIT_OBJECT_0 {
		t.Fatal("the worker's child outlived the worker's job")
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the worker still runs after its job was closed")
	}
}

// waitForChild polls the process list until a child of parent named exe
// appears and returns a handle to it, closed (and the child ended) at cleanup.
func waitForChild(t *testing.T, parent uint32, exe string, limit time.Duration) syscall.Handle {
	t.Helper()
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		pid, ok := findChild(parent, exe)
		if !ok {
			continue
		}
		h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION|syscall.SYNCHRONIZE|processTerminate, false, pid)
		if err != nil {
			t.Fatalf("open %s: %v", exe, err)
		}
		t.Cleanup(func() {
			_ = syscall.TerminateProcess(h, 1)
			_ = syscall.CloseHandle(h)
		})
		return h
	}
	t.Fatalf("no %s child of %d within %s; was the worker resumed?", exe, parent, limit)
	return 0
}

// findChild scans one process snapshot for a child of parent named exe.
func findChild(parent uint32, exe string) (uint32, bool) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, false
	}
	defer syscall.CloseHandle(snap)
	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		if e.ParentProcessID == parent && strings.EqualFold(syscall.UTF16ToString(e.ExeFile[:]), exe) {
			return e.ProcessID, true
		}
	}
	return 0, false
}

// inJob reports whether the process is in the job, failing the test when the
// question cannot be asked.
func inJob(t *testing.T, process, job syscall.Handle) bool {
	t.Helper()
	var in int32
	if r, _, err := procIsProcessInJob.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&in))); r == 0 {
		t.Fatalf("IsProcessInJob: %v", err)
	}
	return in != 0
}
