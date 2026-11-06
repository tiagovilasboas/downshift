// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package workers

import (
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"
)

const (
	jobObjectExtendedLimitInformation = 9
	jobLimitKillOnJobClose            = 0x2000
	processSetQuota                   = 0x0100
	processTerminate                  = 0x0001
	processSuspendResume              = 0x0800
	createSuspended                   = 0x00000004
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject    = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJob  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJob = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject = kernel32.NewProc("TerminateJobObject")

	// NtResumeProcess resumes every thread of a process from its handle. Go's
	// syscall.StartProcess closes the main thread handle CreateProcess returns,
	// so ResumeThread has nothing to act on; the documented alternative is a
	// system-wide Toolhelp32 thread snapshot, four more procs (Thread32First,
	// Thread32Next, OpenThread, ResumeThread) and a THREADENTRY32 mirror, each a
	// way to leave the worker suspended. NtResumeProcess is one call on the
	// handle adoptProcess already opens. It is an ntdll export outside the
	// documented Win32 API, stable since Windows XP (Sysinternals PsSuspend uses
	// it), and ownProcessGroup only suspends a worker when it resolves.
	ntdll               = syscall.NewLazyDLL("ntdll.dll")
	procNtResumeProcess = ntdll.NewProc("NtResumeProcess")
)

// Mirrors JOBOBJECT_EXTENDED_LIMIT_INFORMATION; uintptr fields keep the layout
// right on 32-bit and 64-bit Windows.
type jobBasicLimits struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type jobExtendedLimits struct {
	Basic                 jobBasicLimits
	IoInfo                [6]uint64
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type workerJob struct {
	handle   syscall.Handle
	assigned bool
	// suspended is true when the worker was created suspended and adoptProcess
	// must resume it.
	suspended bool
}

// jobs maps a worker command to its Job Object until killGroup releases it.
var jobs sync.Map // *exec.Cmd -> *workerJob

// ownProcessGroup prepares a Job Object that kills its whole tree when closed,
// and asks for the worker to be created suspended (CREATE_SUSPENDED), the order
// the AssignProcessToJobObject documentation gives: adoptProcess, which must
// follow a successful Start, puts it in the job and only then resumes it, so
// the worker cannot spawn a program outside the job. If the job cannot be made
// the worker starts running and falls back to the tree kill in killGroup; if
// NtResumeProcess is missing it starts running and joins the job after Start.
func ownProcessGroup(cmd *exec.Cmd) {
	h, err := newKillOnCloseJob()
	if err != nil {
		return
	}
	job := &workerJob{handle: h}
	if procNtResumeProcess.Find() == nil {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.CreationFlags |= createSuspended
		job.suspended = true
	}
	jobs.Store(cmd, job)
}

func newKillOnCloseJob() (syscall.Handle, error) {
	r, _, err := procCreateJobObject.Call(0, 0)
	if r == 0 {
		return 0, err
	}
	h := syscall.Handle(r)
	info := jobExtendedLimits{}
	info.Basic.LimitFlags = jobLimitKillOnJobClose
	ok, _, err := procSetInformationJob.Call(r, jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if ok == 0 {
		_ = syscall.CloseHandle(h)
		return 0, err
	}
	return h, nil
}

// adoptProcess puts the started worker in its Job Object so every program it
// spawns belongs to the same tree, then resumes a worker created suspended.
// Call it right after a successful cmd.Start. A failed assignment still
// resumes the worker, which then falls back to the tree kill in killGroup. A
// suspended worker that cannot be opened or resumed is killed instead, so it
// fails visibly rather than hanging suspended.
func adoptProcess(cmd *exec.Cmd) {
	v, ok := jobs.Load(cmd)
	if !ok || cmd.Process == nil {
		return
	}
	job := v.(*workerJob)
	ph, err := syscall.OpenProcess(processSetQuota|processTerminate|processSuspendResume, false, uint32(cmd.Process.Pid))
	if err != nil {
		if job.suspended {
			_ = cmd.Process.Kill()
		}
		return
	}
	defer syscall.CloseHandle(ph)
	if r, _, _ := procAssignProcessToJob.Call(uintptr(job.handle), uintptr(ph)); r != 0 {
		job.assigned = true
	}
	if job.suspended && !resumeProcess(ph) {
		_ = cmd.Process.Kill()
	}
}

// resumeProcess resumes every thread of the process; a negative NTSTATUS is a
// failure.
func resumeProcess(ph syscall.Handle) bool {
	r, _, _ := procNtResumeProcess.Call(uintptr(ph))
	return int32(uint32(r)) >= 0
}

// releaseProcess closes the worker's Job Object once the worker has exited or
// failed to start. Closing it ends anything the worker left running, so no
// orphan outlives the worker, and the handle does not leak.
func releaseProcess(cmd *exec.Cmd) {
	if v, ok := jobs.LoadAndDelete(cmd); ok {
		_ = syscall.CloseHandle(v.(*workerJob).handle)
	}
}

// killGroup stops the worker's whole tree: the Job Object when the worker was
// adopted, otherwise taskkill /T, and the worker alone as the last resort.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if v, ok := jobs.LoadAndDelete(cmd); ok {
		job := v.(*workerJob)
		defer syscall.CloseHandle(job.handle)
		if job.assigned {
			if r, _, _ := procTerminateJobObject.Call(uintptr(job.handle), 1); r != 0 {
				return nil
			}
		}
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	if kill.Run() == nil {
		return nil
	}
	return cmd.Process.Kill()
}
