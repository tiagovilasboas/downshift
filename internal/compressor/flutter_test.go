// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFlutterFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "corpus", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestCompressFlutterTestCompactsCleanTranscripts(t *testing.T) {
	for _, name := range []string{"flutter_pass_compact.txt", "flutter_pass_expanded.txt", "dart_pass_compact.txt", "dart_pass_expanded.txt"} {
		in := readFlutterFixture(t, name)
		if !isFlutterTest(in) {
			t.Fatalf("%s: not detected", name)
		}
		out, applied, reason := compressFlutterTest(in)
		if !applied || !strings.Contains(string(out), "PASS (3 passed, 1 skipped in 00:") || len(out) >= len(in) {
			t.Errorf("%s: applied=%v reason=%q out=%q", name, applied, reason, out)
		}
	}
}

func TestCompressFlutterTestWithoutSkips(t *testing.T) {
	in := []byte("\r00:00 +0: loading t.dart   \r00:01 +0: a   \r00:01 +1: a   \r00:01 +1: All tests passed!   \n")
	out, applied, _ := compressFlutterTest(in)
	if !applied || string(out) != "dart/flutter test: PASS (1 passed in 00:01)\n" {
		t.Fatalf("applied=%v out=%q", applied, out)
	}
}

func TestCompressFlutterTestPreservesAnythingAmbiguous(t *testing.T) {
	const ok = "00:00 +0: loading t.dart\n00:01 +0: a\n00:01 +1: a\n"
	cases := map[string]string{
		"missing trailing newline": ok + "00:01 +1: All tests passed!",
		"crlf":                     strings.ReplaceAll(ok+"00:01 +1: All tests passed!\n", "\n", "\r\n"),
		"ansi":                     ok + "00:01 \x1b[32m+1\x1b[0m: All tests passed!\n",
		"failure counter":          ok + "00:01 +1 -1: b\n00:01 +1 -1: All tests passed!\n",
		"zero failure counter":     ok + "00:01 +1 -0: All tests passed!\n",
		"some tests failed":        ok + "00:01 +1 -1: Some tests failed.\n",
		"error marker":             ok + "00:01 +1: b [E]\n00:01 +1: All tests passed!\n",
		"expected actual":          ok + "Expected: <2>\n  Actual: <1>\n00:01 +1: All tests passed!\n",
		"print output line":        ok + "hello from test\n00:01 +1: All tests passed!\n",
		"print glued to status":    "00:00 +0: a  hello\n00:01 +1: All tests passed!\n",
		"blank line":               ok + "\n00:01 +1: All tests passed!\n",
		"pub get chatter":          "Resolving dependencies...\n" + ok + "00:01 +1: All tests passed!\n",
		"waiting for lock":         "Waiting for another flutter command to release the startup lock...\n" + ok + "00:01 +1: All tests passed!\n",
		"warning":                  ok + "Warning: something\n00:01 +1: All tests passed!\n",
		"no summary":               ok,
		"output after summary":     ok + "00:01 +1: All tests passed!\ntrailing\n",
		"summary twice":            ok + "00:01 +1: All tests passed!\n00:01 +1: All tests passed!\n",
		"zero tests":               "00:00 +0: loading t.dart\n00:00 +0: All tests passed!\n",
		"counter regression":       "00:00 +0: loading t.dart\n00:01 +2: a\n00:01 +1: b\n00:01 +1: All tests passed!\n",
		"skip regression":          "00:00 +0: loading t.dart\n00:01 +1 ~2: a\n00:01 +2 ~1: All tests passed!\n",
		"no tests ran":             "00:00 +0: loading t.dart\n00:00 +0: No tests ran.\n",
		"empty segment":            "\r00:00 +0: loading t.dart\r\r00:01 +1: All tests passed!\n",
	}
	for name, s := range cases {
		in := []byte(s)
		out, applied, reason := compressFlutterTest(in)
		if applied || !bytes.Equal(out, in) || !strings.HasSuffix(reason, "preserved") && reason != "no_compression_gain" {
			t.Errorf("%s: must be preserved byte-identically: applied=%v reason=%q", name, applied, reason)
		}
	}
}

func TestCompressFlutterTestNonASCIIIsPreservedByteIdentically(t *testing.T) {
	for name, text := range map[string]string{
		"zero-width space":  "Err​or",
		"soft hyphen":       "sec­ret",
		"NUL byte":          "Err\x00or",
		"cyrillic e":        "еrror",
		"fullwidth letters": "ＥＲＲＯＲ",
	} {
		in := []byte("00:00 +0: loading t.dart\n00:01 +1: " + text + "\n00:01 +2: All tests passed!\n")
		out, applied, reason := compressFlutterTest(in)
		if applied || !bytes.Equal(out, in) || reason != "flutter_test_non_ascii_text_preserved" {
			t.Errorf("%s: applied=%v reason=%q", name, applied, reason)
		}
	}
}

func TestCompressFlutterTestDangerWordsArePreserved(t *testing.T) {
	for _, w := range []string{"Error: boom", "an Exception", "FAILED to connect", "WARNING secret=x", "panic now", "Fatal", "secret", "api token", "Password"} {
		in := []byte("00:00 +0: loading t.dart\n00:01 +1: " + w + "\n00:01 +2: All tests passed!\n")
		out, applied, reason := compressFlutterTest(in)
		if applied || !bytes.Equal(out, in) || reason != "flutter_test_danger_word_preserved" {
			t.Errorf("%q: applied=%v reason=%q", w, applied, reason)
		}
	}
	for _, s := range []string{
		"00:00 +0: loading t.dart\n00:01 +1: a\r00:02 +2: Err\r00:02 +2: or\r00:02 +2: All tests passed!\n",
		"00:00 +0: loading t.dart\n00:01 +1: a\r00:02 +2: sec\r00:02 +2: ret\r00:02 +2: All tests passed!\n",
	} {
		in := []byte(s)
		out, applied, reason := compressFlutterTest(in)
		if applied || !bytes.Equal(out, in) || reason != "flutter_test_danger_word_preserved" {
			t.Errorf("split danger word must be preserved: applied=%v reason=%q out=%q", applied, reason, out)
		}
	}
	in := []byte("00:00 +0: loading t.dart\n00:01 +1: group parses input\n00:01 +2: group renders widget\n00:01 +2: All tests passed!\n")
	if _, applied, reason := compressFlutterTest(in); !applied {
		t.Errorf("ordinary names must compact: %q", reason)
	}
}

func TestFlutterTestDetectionIsNarrow(t *testing.T) {
	for _, s := range []string{
		"",
		"ok  \texample.com/x\t0.1s\n",
		"PASS\n",
		"00:00 passed\n",
		"Running 3 tests\n00:00 +0: a\n",
		"12:34 +0\n",
	} {
		if isFlutterTest([]byte(s)) {
			t.Errorf("%q must not be detected as flutter test", s)
		}
	}
}

func TestCompressExitCarriageReturnGateOnlyOpensForFlutter(t *testing.T) {
	in := readFlutterFixture(t, "flutter_pass_compact.txt")
	if res := CompressExit(in, ModeSafe, 0); !res.Applied || res.Format != FormatFlutterTest {
		t.Fatalf("clean flutter compact run: applied=%v format=%s reason=%q", res.Applied, res.Format, res.Reason)
	}
	other := []byte("ok  \texample.com/x\t0.1s\r\n")
	if res := CompressExit(other, ModeSafe, 0); res.Applied || res.Reason != "carriage_return_preserved" {
		t.Fatalf("other tool with CR: applied=%v reason=%q", res.Applied, res.Reason)
	}
}
