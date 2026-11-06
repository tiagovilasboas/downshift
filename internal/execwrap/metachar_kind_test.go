// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMetacharKind(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{"cat <<EOF\nhi\nEOF", KindHeredoc},
		{"cat <<EOF > out.txt", KindHeredoc}, // heredoc beats redirect
		{"echo $(date)", KindSubst},
		{"echo `date`", KindSubst},
		{"echo ${HOME}", KindSubst},
		{"ls $DIR", KindSubst},
		{"echo $(date) > out", KindSubst}, // subst beats redirect
		{"go test ./... > out.txt", KindRedirect},
		{"cat < in.txt", KindRedirect},
		{"go test ./... 2>/dev/null", KindRedirect},
		{"go test ./... 2>&1", KindOther},
		{"go test ./... 2>&1 | tail", KindRedirect}, // only a trailing 2>&1 is exempt
		{"git log || true", KindOrList},
		{"a || b | grep x", KindOrList}, // or_list beats pipe
		{"sleep 1 &", KindBackground},
		{"a && b", KindOther},
		{"go test | grep FAIL", KindPipeGrep},
		{"go test | rg FAIL", KindPipeGrep},
		{"go test | egrep x", KindPipeGrep},
		{"go test | fgrep x", KindPipeGrep},
		{"go test | tail -5 | grep x", KindPipeGrep},
		{"go test | tail -5", KindPipeOther},
		{`grep "a|b" file`, KindQuote}, // pipe inside quotes is no pipe
		{"echo 'hi'", KindQuote},
		{"ls *.go", KindGlob},
		{"cat ~/x", KindGlob},
		{"ls file?.txt", KindGlob},
		{"a; b", KindOther},
		{"", KindOther},
	}
	for _, c := range cases {
		if got := MetacharKind(c.cmd); got != c.want {
			t.Errorf("MetacharKind(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestMetacharKindsCoverEveryCode(t *testing.T) {
	if len(MetacharKinds) != 10 {
		t.Fatalf("MetacharKinds has %d codes, want 10", len(MetacharKinds))
	}
}

func TestSkipKindLogHoldsNoCommandText(t *testing.T) {
	p := filepath.Join(t.TempDir(), SkipsFileName)
	cmd := "cat /Users/someone/secret.txt | grep hunter2"
	ev := SkipEvent{Reason: SkipMetachar, Harness: "claude-code", Kind: MetacharKind(cmd)}
	if err := AppendSkipEvent(p, ev, time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"secret", "hunter2", "/Users", "cat "} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("skip row leaks %q: %s", leak, raw)
		}
	}
	kinds, _ := SummarizeSkipKinds(p)
	if kinds[KindPipeGrep] != 1 {
		t.Errorf("kinds = %v, want one pipe_grep", kinds)
	}
}
