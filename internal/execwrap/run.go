// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
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

// Options configures one Run.
type Options struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	// Compact allows compaction. False runs the command as a plain
	// passthrough (the DOWNSHIFT_EXEC=0 kill switch).
	Compact bool
	// Signals, when set, replaces signal.Notify as the source of signals to
	// forward (tests).
	Signals <-chan os.Signal
}

// Result is what one Run did. It holds no command text and no output.
type Result struct {
	ExitCode     int
	ExitClass    string
	Family       Family
	RawBytes     int64 // stdout plus stderr the program wrote
	EmittedBytes int64 // stdout plus stderr the wrapper wrote
	Compacted    bool
}

// expectedFormat is the compressor format each family may be compacted as.
// Any other detected format (a repetitive-log match, a search match) keeps the
// raw output.
var expectedFormat = map[Family]compressor.Format{
	FamilyGoTest:     compressor.FormatGoTest,
	FamilyGitStatus:  compressor.FormatGitStatus,
	FamilyPytest:     compressor.FormatPytest,
	FamilyVitest:     compressor.FormatVitest,
	FamilyPlaywright: compressor.FormatPlaywright,
	FamilyCargoTest:  compressor.FormatCargoTest,
	FamilyTSC:        compressor.FormatTSC,
	FamilyJest:       compressor.FormatJest,
	FamilyCypress:    compressor.FormatCypress,
	FamilyNextBuild:  compressor.FormatNextBuild,
	FamilyPHPUnit:    compressor.FormatPHPUnit,
	FamilyMaven:      compressor.FormatMaven,
	FamilyGradle:     compressor.FormatGradle,
}

// Run executes argv directly (no shell) with the caller's stdin, and exits
// with the program's own status. Only the allowlisted families run (anything
// else is refused with 126). Output keeps the order the program wrote it in.
// With compaction on, stdout is held (see gate) and compacted only
// when the program exited 0, wrote nothing on stderr, stayed under
// compressor.MaxProcessableBytes and HoldBound (when configured), and the
// compressor recognised the family's own format. Otherwise it is byte-identical.
// Any internal error prints the raw output: output is never lost.
func Run(argv []string, o Options) (res Result) {
	if len(argv) == 0 {
		fmt.Fprintln(o.Stderr, "downshift compact: no command (usage: downshift compact -- <program> [args...])")
		return Result{ExitCode: 2, ExitClass: ExitClassStartError}
	}
	res.Family = FamilyOf(argv)
	if res.Family == FamilyNone {
		// exec is not a generic runner: only the allowlisted families run, so
		// a wrapped command can never be anything the hook would not wrap.
		fmt.Fprintln(o.Stderr, "downshift compact: refused: only allowlisted test, build, and status commands may run through exec")
		return Result{ExitCode: ExitCannotExecute, ExitClass: ExitClassRefused}
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return startFailure(argv[0], err, o.Stderr, res.Family)
	}

	g := newGate(o.Stdout, o.Stderr, o.Compact)
	cmd := exec.Command(path, argv[1:]...)
	cmd.Args[0] = argv[0]
	cmd.Env = append(os.Environ(), DepthEnv+"=1")
	ownProcessGroup(cmd)
	cmd.Stdin = o.Stdin
	cmd.Stdout = stdoutWriter{g}
	cmd.Stderr = stderrWriter{g}

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
	res.RawBytes, res.EmittedBytes, res.Compacted = g.finish(res.ExitCode == 0, expectedFormat[res.Family])
	return res
}

// startFailure answers like a shell when the program cannot start.
func startFailure(name string, err error, stderr io.Writer, family Family) Result {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "downshift compact: %s: command not found\n", name)
		return Result{ExitCode: ExitNotFound, ExitClass: ExitClassNotFound, Family: family}
	}
	fmt.Fprintf(stderr, "downshift compact: %s: %v\n", name, err)
	return Result{ExitCode: ExitCannotExecute, ExitClass: ExitClassStartError, Family: family}
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

// HoldBound, when positive, bounds how long stdout of an allowlisted family
// is held while it could still be compacted. Past it the held bytes are flushed
// and the rest streams. By default it is 0 (disabled), so batch commands like
// `go test` hold the buffer until process completion without arbitrary timeouts.
var HoldBound time.Duration

// gate serialises the program's two output streams into the caller's two
// writers so their relative order is preserved. Stdout is held back only
// while compaction is still possible: the first stderr byte, the size limit
// or HoldBound (when positive) releases the held bytes first, in write order,
// and from then on both streams pass straight through and nothing is compacted.
type gate struct {
	mu        sync.Mutex
	out, err  io.Writer
	buf       bytes.Buffer
	streaming bool
	timer     *time.Timer
	nOut      int64
	nErr      int64
}

func newGate(out, err io.Writer, hold bool) *gate {
	return &gate{out: out, err: err, streaming: !hold}
}

// release flushes held stdout and switches to streaming. Callers hold g.mu.
func (g *gate) release() {
	if g.streaming {
		return
	}
	g.streaming = true
	if g.timer != nil {
		g.timer.Stop()
	}
	_, _ = g.out.Write(g.buf.Bytes()) // a closed reader must not kill the program
	g.buf = bytes.Buffer{}
}

type stdoutWriter struct{ g *gate }

func (w stdoutWriter) Write(p []byte) (int, error) {
	g := w.g
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nOut += int64(len(p))
	if !g.streaming && g.buf.Len()+len(p) > compressor.MaxProcessableBytes {
		g.release()
	}
	if g.streaming {
		_, _ = g.out.Write(p)
		return len(p), nil
	}
	if g.buf.Len() == 0 && HoldBound > 0 {
		g.timer = time.AfterFunc(HoldBound, func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.release()
		})
	}
	g.buf.Write(p)
	return len(p), nil
}

type stderrWriter struct{ g *gate }

func (w stderrWriter) Write(p []byte) (int, error) {
	g := w.g
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nErr += int64(len(p))
	g.release()
	_, _ = g.err.Write(p)
	return len(p), nil
}

// finish writes any still-held stdout, compacted when the exit code was 0,
// nothing was released early and the compressor applied the expected format,
// raw otherwise. It returns the raw and emitted byte counts.
func (g *gate) finish(exitZero bool, want compressor.Format) (raw, emitted int64, compacted bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	raw = g.nOut + g.nErr
	if g.streaming {
		return raw, raw, false
	}
	if g.timer != nil {
		g.timer.Stop()
	}
	data := g.buf.Bytes()
	g.streaming = true // a timer that fires after finish finds nothing to flush
	g.buf = bytes.Buffer{}
	out := data
	if exitZero && want != "" {
		if c, ok := compactSafely(data, want); ok {
			out, compacted = c, true
		}
	}
	_, _ = g.out.Write(out)
	return raw, int64(len(out)), compacted
}

// compactSafely runs the compressor under recover: a compressor bug prints
// the raw output instead.
func compactSafely(data []byte, want compressor.Format) (out []byte, ok bool) {
	defer func() {
		if recover() != nil {
			out, ok = nil, false
		}
	}()
	r := compressor.CompressExit(data, compressor.ModeSafe, 0)
	if !r.Applied || r.Format != want {
		return nil, false
	}
	out = append(append([]byte{}, r.Output...), Marker(len(data), len(r.Output))...)
	if len(out) >= len(data) {
		return nil, false
	}
	return out, true
}

// Marker is the last line of compacted output. It tells the reader that the
// text above is a summary, not what the program printed.
func Marker(rawBytes, summaryBytes int) string {
	return fmt.Sprintf("[downshift compact: compacted %d to %d bytes]\n", rawBytes, summaryBytes)
}
