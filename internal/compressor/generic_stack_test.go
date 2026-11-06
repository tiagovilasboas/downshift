// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Each shape returns the lines of frame i.
var stackShapes = map[string]func(i int) []string{
	"js": func(i int) []string {
		return []string{fmt.Sprintf("    at fn%d (/app/src/f%d.js:%d:7)", i, i, i+1)}
	},
	"java": func(i int) []string {
		return []string{fmt.Sprintf("\tat com.acme.Svc%d.run(Svc%d.java:%d)", i, i, i+10)}
	},
	"python": func(i int) []string {
		return []string{fmt.Sprintf("  File \"/app/m%d.py\", line %d, in f%d", i, i+1, i), fmt.Sprintf("    call_%d()", i)}
	},
	"go": func(i int) []string {
		return []string{fmt.Sprintf("example.com/app/pkg.fn%d(0xc000012345)", i), fmt.Sprintf("\t/app/pkg/f%d.go:%d +0x1d", i, i+3)}
	},
}

func stackText(shape string, n int) (text string, frames [][]string) {
	var b strings.Builder
	b.WriteString("caller dump follows\n")
	for i := 0; i < n; i++ {
		f := stackShapes[shape](i)
		frames = append(frames, f)
		b.WriteString(strings.Join(f, "\n") + "\n")
	}
	b.WriteString("done\n")
	return b.String(), frames
}

func joinFrames(fs [][]string) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString(strings.Join(f, "\n") + "\n")
	}
	return b.String()
}

func TestStackFoldEachShape(t *testing.T) {
	for shape := range stackShapes {
		t.Run(shape, func(t *testing.T) {
			in, frames := stackText(shape, 40)
			out, ok, _ := compressGeneric([]byte(in))
			want := "caller dump follows\n" + joinFrames(frames[:5]) +
				"[30 frames elided]\n" + joinFrames(frames[35:]) + "done\n"
			if !ok || string(out) != want {
				t.Fatalf("ok=%v\n got: %q\nwant: %q", ok, out, want)
			}
		})
	}
}

func TestStackFoldBoundary(t *testing.T) {
	for shape := range stackShapes {
		for _, c := range []struct {
			n      int
			folded bool
		}{{12, false}, {13, true}} {
			in, _ := stackText(shape, c.n)
			out, ok, _ := compressGeneric([]byte(in))
			if ok != c.folded {
				t.Errorf("%s n=%d: applied=%v want %v", shape, c.n, ok, c.folded)
			}
			if !ok && string(out) != in {
				t.Errorf("%s n=%d: short run changed", shape, c.n)
			}
			if ok && !strings.Contains(string(out), "[3 frames elided]") {
				t.Errorf("%s n=%d: want [3 frames elided], got %q", shape, c.n, out)
			}
		}
	}
}

func TestStackFoldKeepsRawWhenGateFires(t *testing.T) {
	cases := map[string]func(in string) string{
		"message line": func(in string) string { return strings.Replace(in, "caller dump follows", "Error: boom", 1) },
		"line after":   func(in string) string { return in + "1 failed\n" },
		"unknown at line": func(in string) string {
			return in + "    at weird thing:12\n"
		},
	}
	for name, mut := range cases {
		in, _ := stackText("js", 40)
		in = mut(in)
		if out, ok, _ := compressGeneric([]byte(in)); ok || string(out) != in {
			t.Errorf("%s: not kept raw", name)
		}
	}
}

func TestStackFoldInterleavedLineBreaksRun(t *testing.T) {
	in, frames := stackText("java", 20)
	lines := strings.Split(in, "\n")
	// 20 frames, a plain line after the 10th: two runs of 10, none foldable.
	mid := append(append([]string{}, lines[:11]...), "marker line")
	mid = append(mid, lines[11:]...)
	raw := strings.Join(mid, "\n")
	if out, ok, _ := compressGeneric([]byte(raw)); ok || string(out) != raw {
		t.Fatalf("two short runs must stay raw, got ok=%v", ok)
	}
	_ = frames
}

func TestStackFoldMixedKindsDoNotJoin(t *testing.T) {
	js, _ := stackText("js", 7)
	java, _ := stackText("java", 7)
	raw := strings.TrimSuffix(js, "done\n") + strings.TrimPrefix(java, "caller dump follows\n")
	if out, ok, _ := compressGeneric([]byte(raw)); ok || string(out) != raw {
		t.Fatal("7 + 7 frames of two kinds must not fold as one run")
	}
}

// The race detector slows the regex loop about 20x, so the size stays modest;
// a quadratic fold would still blow the bound at 40k frames.
func TestStackFoldLinearOnHugeInput(t *testing.T) {
	in, _ := stackText("go", 40000)
	start := time.Now()
	out, ok, _ := compressGeneric([]byte(in))
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("40k frames took %v", d)
	}
	if !ok || !strings.Contains(string(out), "[39990 frames elided]") {
		t.Fatalf("huge run not folded: ok=%v", ok)
	}
}

func TestStackFoldMarkerInInputKeepsRaw(t *testing.T) {
	in, _ := stackText("js", 40)
	in += "[5 frames elided]\n"
	if out, ok, _ := compressGeneric([]byte(in)); ok || string(out) != in {
		t.Fatal("input that already looks like a marker must stay raw")
	}
}
