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
	"syscall"

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
	FamilyGoTest:    compressor.FormatGoTest,
	FamilyGitStatus: compressor.FormatGitStatus,
}

// Run executes argv directly (no shell) with the caller's stdin, and exits
// with the program's own status. Stderr always streams through unchanged.
// Stdout streams through unchanged too unless argv is an allowlisted family
// and compaction is on; then it is held in memory up to
// compressor.MaxProcessableBytes and compacted only when the program exited 0,
// wrote nothing on stderr, and the compressor recognised the family's own
// format. Past the bound, the held bytes are flushed and the rest streams.
// Any internal error prints the raw output: output is never lost.
func Run(argv []string, o Options) (res Result) {
	if len(argv) == 0 {
		fmt.Fprintln(o.Stderr, "downshift exec: no command (usage: downshift exec -- <program> [args...])")
		return Result{ExitCode: 2, ExitClass: ExitClassStartError}
	}
	res.Family = FamilyOf(argv)
	if res.Family == FamilyNone {
		// exec is not a generic runner: only the allowlisted families run, so
		// a wrapped command can never be anything the hook would not wrap.
		fmt.Fprintln(o.Stderr, "downshift exec: refused: only `go test` and `git status` may run through exec")
		return Result{ExitCode: ExitCannotExecute, ExitClass: ExitClassRefused}
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return startFailure(argv[0], err, o.Stderr, res.Family)
	}

	stderr := &countingWriter{w: o.Stderr}
	var held *heldWriter
	cmd := exec.Command(path, argv[1:]...)
	cmd.Args[0] = argv[0]
	ownProcessGroup(cmd)
	cmd.Stdin = o.Stdin
	cmd.Stderr = stderr
	if o.Compact && res.Family != FamilyNone {
		held = &heldWriter{out: o.Stdout, limit: compressor.MaxProcessableBytes}
		cmd.Stdout = held
	} else {
		cmd.Stdout = &countingWriter{w: o.Stdout}
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
	res.RawBytes = stderr.n
	res.EmittedBytes = stderr.n
	switch w := cmd.Stdout.(type) {
	case *countingWriter:
		res.RawBytes += w.n
		res.EmittedBytes += w.n
	case *heldWriter:
		raw, emitted, compacted := w.finish(res.ExitCode == 0 && stderr.n == 0, expectedFormat[res.Family])
		res.RawBytes += raw
		res.EmittedBytes += emitted
		res.Compacted = compacted
	}
	return res
}

// startFailure answers like a shell when the program cannot start.
func startFailure(name string, err error, stderr io.Writer, family Family) Result {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stderr, "downshift exec: %s: command not found\n", name)
		return Result{ExitCode: ExitNotFound, ExitClass: ExitClassNotFound, Family: family}
	}
	fmt.Fprintf(stderr, "downshift exec: %s: %v\n", name, err)
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

// countingWriter passes bytes through and counts them.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	_, _ = c.w.Write(p) // a closed reader must not kill the program
	return len(p), nil
}

// heldWriter holds stdout up to limit bytes. Past the limit it flushes what it
// held and streams the rest, so memory stays bounded and nothing is lost.
type heldWriter struct {
	out       io.Writer
	limit     int
	buf       bytes.Buffer
	streaming bool
	n         int64
}

func (h *heldWriter) Write(p []byte) (int, error) {
	h.n += int64(len(p))
	if !h.streaming && h.buf.Len()+len(p) > h.limit {
		h.streaming = true
		_, _ = h.out.Write(h.buf.Bytes())
		h.buf = bytes.Buffer{}
	}
	if h.streaming {
		_, _ = h.out.Write(p)
		return len(p), nil
	}
	h.buf.Write(p)
	return len(p), nil
}

// finish writes the held stdout, compacted when clean is true and the
// compressor applied the expected format, raw otherwise. It returns raw and
// emitted byte counts.
func (h *heldWriter) finish(clean bool, want compressor.Format) (raw, emitted int64, compacted bool) {
	if h.streaming {
		return h.n, h.n, false
	}
	data := h.buf.Bytes()
	out := data
	if clean && want != "" {
		if c, ok := compactSafely(data, want); ok {
			out, compacted = c, true
		}
	}
	_, _ = h.out.Write(out)
	return h.n, int64(len(out)), compacted
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
	return fmt.Sprintf("[downshift exec: compacted %d to %d bytes]\n", rawBytes, summaryBytes)
}
