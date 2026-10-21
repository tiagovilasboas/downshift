// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

// --- cargo build -----------------------------------------------------------

func TestCompressCargoBuildErrorsAndWarnings(t *testing.T) {
	in := "" +
		"   Compiling myapp v0.1.0 (/path/to/myapp)\n" +
		"warning: unused variable: `x`\n" +
		"  --> src/main.rs:4:9\n" +
		"   |\n" +
		"4  |     let x = 5;\n" +
		"   |         ^ help: prefix it with an underscore\n" +
		"   |\n" +
		"   = note: `#[warn(unused_variables)]` on by default\n" +
		"\n" +
		"error[E0308]: mismatched types\n" +
		"  --> src/lib.rs:10:5\n" +
		"   |\n" +
		"10 |     return 1;\n" +
		"   |            ^ expected `String`, found integer\n" +
		"\n" +
		"error: aborting due to 1 previous error; 1 warning emitted\n"

	out, applied, reason := compressCargoBuild([]byte(in))
	if !applied {
		t.Fatalf("expected compression, got reason %q", reason)
	}
	got := string(out)
	for _, want := range []string{
		"[ ] cargo build: 1 error, 1 warning\n",
		"src/main.rs:\n",
		"warning: unused variable: `x`",
		"src/lib.rs:\n",
		"error[E0308]: mismatched types",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
	// Progress and code frames are dropped.
	for _, drop := range []string{"Compiling", "let x = 5", "= note", "aborting due to"} {
		if strings.Contains(got, drop) {
			t.Errorf("output should not contain %q\n---\n%s", drop, got)
		}
	}
	if len(out) >= len(in) {
		t.Errorf("output not shorter: %d -> %d", len(in), len(out))
	}
}

func TestCompressCargoBuildPreserved(t *testing.T) {
	base := "" +
		"warning: unused variable: `x`\n" +
		"  --> src/main.rs:4:9\n" +
		"   = note: on by default\n"
	for name, in := range map[string]string{
		"no diagnostics": "   Compiling myapp v0.1.0\n    Finished dev in 2.34s\n",
		"no newline":     strings.TrimSuffix(base, "\n"),
		"crlf":           strings.ReplaceAll(base, "\n", "\r\n"),
		"unrecognized":   base + "some stray linker noise here\n",
		"empty":          "",
	} {
		out, applied, reason := compressCargoBuild([]byte(in))
		if applied || string(out) != in || !strings.HasPrefix(reason, "cargo_build_") {
			t.Errorf("%s: applied=%v reason=%q", name, applied, reason)
		}
	}
}

// --- go build --------------------------------------------------------------

func TestCompressGoBuildErrors(t *testing.T) {
	in := "" +
		"go: downloading github.com/foo/bar v1.2.3\n" +
		"# example.com/app/internal/core\n" +
		"internal/core/parse.go:12:5: undefined: Foo\n" +
		"internal/core/parse.go:20:2: cannot use x (untyped int constant) as string value\n" +
		"cmd/app/main.go:3:8: \"fmt\" imported and not used\n"

	out, applied, reason := compressGoBuild([]byte(in))
	if !applied {
		t.Fatalf("expected compression, got reason %q", reason)
	}
	got := string(out)
	for _, want := range []string{
		"[ ] go build: 3 errors, 0 warnings\n",
		"internal/core/parse.go:\n",
		"undefined: Foo",
		"cannot use x",
		"cmd/app/main.go:\n",
		"imported and not used",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
	for _, drop := range []string{"go: downloading", "# example.com"} {
		if strings.Contains(got, drop) {
			t.Errorf("output should not contain %q\n---\n%s", drop, got)
		}
	}
	if len(out) >= len(in) {
		t.Errorf("output not shorter: %d -> %d", len(in), len(out))
	}
}

func TestCompressGoBuildPreserved(t *testing.T) {
	base := "internal/core/parse.go:12:5: undefined: Foo\n"
	for name, in := range map[string]string{
		"no errors":    "go: downloading github.com/foo/bar v1.2.3\n",
		"no newline":   strings.TrimSuffix(base, "\n"),
		"crlf":         strings.ReplaceAll(base, "\n", "\r\n"),
		"unrecognized": base + "runtime: panic before goroutine stack expand\n",
		"empty":        "",
	} {
		out, applied, reason := compressGoBuild([]byte(in))
		if applied || string(out) != in || !strings.HasPrefix(reason, "go_build_") {
			t.Errorf("%s: applied=%v reason=%q", name, applied, reason)
		}
	}
}

// --- dotnet build ----------------------------------------------------------

func TestCompressDotnetBuildErrorsAndWarnings(t *testing.T) {
	in := "" +
		"Determining projects to restore...\n" +
		"  Restored /path/App.csproj (in 1.2 sec).\n" +
		"  App -> /path/bin/Debug/net8.0/App.dll\n" +
		"/path/Program.cs(12,5): error CS0103: The name 'x' does not exist [/path/App.csproj]\n" +
		"/path/Program.cs(20,9): warning CS0168: The variable 'y' is declared but never used [/path/App.csproj]\n" +
		"/path/Helper.cs(3,1): error CS0246: The type 'Foo' could not be found [/path/App.csproj]\n" +
		"Build FAILED.\n" +
		"    1 Warning(s)\n" +
		"    2 Error(s)\n" +
		"Time Elapsed 00:00:01.23\n"

	out, applied, reason := compressDotnetBuild([]byte(in))
	if !applied {
		t.Fatalf("expected compression, got reason %q", reason)
	}
	got := string(out)
	for _, want := range []string{
		"[ ] dotnet build: 2 errors, 1 warning\n",
		"/path/Program.cs:\n",
		"error CS0103",
		"warning CS0168",
		"/path/Helper.cs:\n",
		"error CS0246",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n---\n%s", want, got)
		}
	}
	// The trailing [project] tag, restore banner, "->" line and summary drop.
	for _, drop := range []string{"[/path/App.csproj]", "Determining projects", "-> /path/bin", "Build FAILED", "Time Elapsed"} {
		if strings.Contains(got, drop) {
			t.Errorf("output should not contain %q\n---\n%s", drop, got)
		}
	}
	if len(out) >= len(in) {
		t.Errorf("output not shorter: %d -> %d", len(in), len(out))
	}
}

func TestCompressDotnetBuildPreserved(t *testing.T) {
	base := "/path/Program.cs(12,5): error CS0103: The name 'x' does not exist [/path/App.csproj]\n"
	for name, in := range map[string]string{
		"no diagnostics": "Determining projects to restore...\n  Restored /path/App.csproj (in 1.2 sec).\n",
		"no newline":     strings.TrimSuffix(base, "\n"),
		"crlf":           strings.ReplaceAll(base, "\n", "\r\n"),
		"unrecognized":   base + "MSB4018: The target threw an unexpected exception.\n",
		"empty":          "",
	} {
		out, applied, reason := compressDotnetBuild([]byte(in))
		if applied || string(out) != in || !strings.HasPrefix(reason, "dotnet_build_") {
			t.Errorf("%s: applied=%v reason=%q", name, applied, reason)
		}
	}
}
