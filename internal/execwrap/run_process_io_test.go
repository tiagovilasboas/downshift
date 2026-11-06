// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

func TestRun_StdinInherited(t *testing.T) {
	fakeBin(t, "go", "cat")
	var out bytes.Buffer
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader("hello\n"), Stdout: &out, Stderr: &bytes.Buffer{}, Compact: true})
	if r.ExitCode != 0 || out.String() != "hello\n" {
		t.Fatalf("exit=%d out=%q", r.ExitCode, out.String())
	}
}

func TestRun_NonUTF8Passthrough(t *testing.T) {
	// Bytes 0xff 0xfe plus a NUL: the compressor refuses them, output is identical.
	fakeBin(t, "go", "printf 'ok  x 0.1s\\nok  y 0.1s\\n\\377\\376\\000\\n'")
	r, out, _ := run(t, []string{"go", "test"}, true)
	want := "ok  x 0.1s\nok  y 0.1s\n\xff\xfe\x00\n"
	if r.Compacted || out != want {
		t.Fatalf("compacted=%v out=%q", r.Compacted, out)
	}
}

func TestRun_LargeOutputStreamsRaw(t *testing.T) {
	// More than MaxProcessableBytes of passing lines: held bytes flush, the
	// rest streams, and the output is byte-identical and not compacted.
	n := compressor.MaxProcessableBytes/len("ok  github.com/x/pkg 0.1s\n") + 100
	fakeBin(t, "go", "i=0; while [ $i -lt "+itoa(n)+" ]; do echo 'ok  github.com/x/pkg 0.1s'; i=$((i+1)); done")
	r, out, _ := run(t, []string{"go", "test"}, true)
	want := strings.Repeat("ok  github.com/x/pkg 0.1s\n", n)
	if r.Compacted || out != want {
		t.Fatalf("compacted=%v len(out)=%d want %d", r.Compacted, len(out), len(want))
	}
	if r.RawBytes != int64(len(want)) || r.EmittedBytes != int64(len(want)) {
		t.Fatalf("raw=%d emitted=%d want %d", r.RawBytes, r.EmittedBytes, len(want))
	}
}

func TestRun_SignalForwarded(t *testing.T) {
	fakeBin(t, "go", "exec sleep 10")
	sig := make(chan os.Signal, 1)
	var out, errb bytes.Buffer
	go func() {
		time.Sleep(300 * time.Millisecond)
		sig <- syscall.SIGTERM
	}()
	start := time.Now()
	r := Run([]string{"go", "test"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, Signals: sig})
	if r.ExitCode != 128+int(syscall.SIGTERM) || r.ExitClass != ExitClassSignal {
		t.Fatalf("exit = %d/%s, want %d/signal", r.ExitCode, r.ExitClass, 128+int(syscall.SIGTERM))
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("signal was not forwarded: child ran to completion")
	}
}
