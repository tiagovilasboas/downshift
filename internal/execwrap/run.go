// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Exit codes a POSIX shell uses when it cannot run a program.
const (
	ExitNotFound      = 127
	ExitCannotExecute = 126
)

// Exit-code classes recorded in telemetry.
const (
	ExitClassZero       = "zero"
	ExitClassNonZero    = "nonzero"
	ExitClassSignal     = "signal"
	ExitClassNotFound   = "not_found"
	ExitClassStartError = "start_error"
	ExitClassRefused    = "refused"
)

// forwardedSignals are relayed to the child so an interrupt or termination of
// the wrapper reaches the program it runs.
var forwardedSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT}

// DepthEnv is set to "1" in the child's environment. A wrapper that finds it
// set runs as a plain passthrough and the hook does not wrap again, so a
// program that itself runs commands never nests compaction.
const DepthEnv = "DOWNSHIFT_EXEC_DEPTH"

// KillSwitchEnv turns the exec wrapper off everywhere when set to "0".
const KillSwitchEnv = "DOWNSHIFT_EXEC"

// Options configures one Run.
type Options struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	// Compact allows compaction. False runs the command as a plain
	// passthrough (the DOWNSHIFT_EXEC=0 kill switch).
	Compact bool
	// SkipObservation keeps synthetic commands out of learning.
	SkipObservation bool
	// Keep, when set, receives the raw output of a run that was really
	// compacted and may store it; ok makes the marker carry the id. It never
	// fails the run: a panic or false keeps the plain marker.
	Keep func(raw []byte) (id string, ok bool)
	// KeepFailed is Keep for a run that exited non-zero only, used when Keep
	// is nil: CCR on failure, successes are not stored.
	KeepFailed func(raw []byte) (id string, ok bool)
	// Signals, when set, replaces signal.Notify as the source of signals to
	// forward (tests).
	Signals <-chan os.Signal
	// DiscardStderr sends the program's stderr to the null device (a trailing
	// 2>/dev/null); ExitZero makes ShellExit return 0 (a trailing || true).
	// Each adds a note at the end of a compacted output (run_tail.go).
	DiscardStderr, ExitZero bool
}

// Result is what one Run did. It holds no command text and no output.
type Result struct {
	ExitCode     int
	ExitClass    string
	Family       Family
	RawBytes     int64 // stdout plus stderr the program wrote
	EmittedBytes int64 // stdout plus stderr the wrapper wrote
	Compacted    bool
	// Folded is true when the compacted output is a lossy fold with the raw
	// kept in CCR, not the lossless compressor. Compacted is true as well.
	Folded bool
	// Passthrough is the Passthrough* reason when Compacted is false.
	Passthrough string
	// PassthroughDetail is the Detail* code refining PassthroughUnrecognized;
	// empty for every other reason.
	PassthroughDetail string
}

// ObserveUncompactedHook is called when a command's output was not compacted.
// It receives the argv, raw output (truncated), and family. Callers register
// a function to learn from uncompacted outputs. The hook must be fast and
// must not fail the command.
var ObserveUncompactedHook func(argv []string, sample []byte, family Family)

// The set of compressor formats each family may be compacted as lives in
// expected_format.go (expectedFormats). A family with no entry there is a
// pass-through: its output is never compacted.

// Run executes argv directly (no shell) with the caller's stdin, and exits
// with the program's own status. Only the allowlisted families run (anything
// else is refused with 126). Output keeps the order the program wrote it in.
// With compaction on, stdout is held (see gate) and compacted only
// when the program wrote nothing on stderr (except families in stderrIsReport,
// whose reporter writes there) and exited 0, or exited non-zero in a family
// CompressFailed supports, which keeps every failure; stayed under
// compressor.MaxProcessableBytes and HoldBound (when configured), and the
// compressor recognised the family's own format. Otherwise it is byte-identical.
// Any internal error prints the raw output: output is never lost.
func Run(argv []string, o Options) (res Result) {
	if len(argv) == 0 {
		fmt.Fprintln(o.Stderr, "downshift compact: no command (usage: downshift compact -- <program> [args...])")
		return Result{ExitCode: 2, ExitClass: ExitClassStartError, Passthrough: PassthroughNonZeroExit}
	}
	defer func() {
		// Only a compacted output gets the notes, as it gets the Marker;
		// any other stdout stays byte-identical to the program's.
		if res.Compacted {
			writeTailNotes(o.Stdout, o, res.ExitCode)
		}
	}()
	res.Family = FamilyOf(argv)
	if res.Family == FamilyNone {
		// exec is not a generic runner: only the allowlisted families run, so
		// a wrapped command can never be anything the hook would not wrap.
		fmt.Fprintf(o.Stderr, "downshift compact: refused: not allowlisted. Allowed: %s; run the command without downshift compact\n", AllowlistSummary())
		return Result{ExitCode: ExitCannotExecute, ExitClass: ExitClassRefused, Passthrough: PassthroughRefused}
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return startFailure(argv[0], err, o.Stderr, res.Family)
	}

	g := newGate(o.Stdout, o.Stderr, o.Compact && readCompactible(res.Family, argv), argv, res.Family)
	g.skipObservation = o.SkipObservation
	g.keep, g.keepFailed = o.Keep, o.KeepFailed
	cmd := exec.Command(path, argv[1:]...)
	cmd.Args[0] = argv[0]
	cmd.Env = append(os.Environ(), DepthEnv+"=1")
	ownProcessGroup(cmd)
	cmd.Stdin = o.Stdin
	cmd.Stdout = stdoutWriter{g}
	cmd.Stderr = stderrWriter{g}
	if o.DiscardStderr {
		cmd.Stderr = nil // os/exec connects a nil Stderr to the null device
	}

	signals := o.Signals
	if signals == nil {
		ch := make(chan os.Signal, 4)
		signal.Notify(ch, forwardedSignals...)
		defer signal.Stop(ch)
		signals = ch
	}
	if err := cmd.Start(); err != nil {
		return startFailure(argv[0], err, o.Stderr, res.Family)
	}
	release := watchGroup(cmd)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case s := <-signals:
				signalChild(cmd, s)
			case <-done:
				return
			}
		}
	}()

	res.ExitCode, res.ExitClass = exitStatus(cmd.Wait())
	release()
	res.RawBytes, res.EmittedBytes, res.Compacted, res.Folded, res.Passthrough = g.finish(res.ExitCode, res.Family)
	res.PassthroughDetail = g.detail
	return res
}

// startFailure answers like a shell when the program cannot start. The exit
// code is non-zero, so the reason is a non-zero exit.
func startFailure(name string, err error, stderr io.Writer, family Family) Result {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "downshift compact: %s: command not found\n", name)
		return Result{ExitCode: ExitNotFound, ExitClass: ExitClassNotFound, Family: family, Passthrough: PassthroughNonZeroExit}
	}
	fmt.Fprintf(stderr, "downshift compact: %s: %v\n", name, err)
	return Result{ExitCode: ExitCannotExecute, ExitClass: ExitClassStartError, Family: family, Passthrough: PassthroughNonZeroExit}
}

// exitStatus maps a Wait error to the shell's exit status: the program's own
// code, or 128+N when signal N killed it.
func exitStatus(err error) (int, string) {
	if err == nil {
		return 0, ExitClassZero
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return ExitCannotExecute, ExitClassStartError
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal()), ExitClassSignal
	}
	code := exitErr.ExitCode()
	if code < 0 {
		return ExitCannotExecute, ExitClassStartError
	}
	return code, ExitClassNonZero
}
