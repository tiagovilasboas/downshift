// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBudgetTokens(t *testing.T) {
	cases := []struct{ user, file, want int64 }{{0, 0, 0}, {0, 500, 500}, {900, 500, 900}, {900, 0, 900}}
	for _, c := range cases {
		if got := ResolveBudgetTokens(c.user, c.file); got != c.want {
			t.Fatalf("Resolve(%d,%d) = %d, want %d", c.user, c.file, got, c.want)
		}
	}
}

func TestEnvBudgetTokens(t *testing.T) {
	cases := []struct {
		raw   string
		want  int64
		notes int
	}{{"", 0, 0}, {"1200", 1200, 0}, {" 7 ", 7, 0}, {"0", 0, 1}, {"-3", 0, 1}, {"1.5", 0, 1}, {"lots", 0, 1}}
	for _, c := range cases {
		n, notes := EnvBudgetTokens(func(string) string { return c.raw })
		if n != c.want || len(notes) != c.notes {
			t.Fatalf("Env(%q) = %d,%v want %d,%d notes", c.raw, n, notes, c.want, c.notes)
		}
	}
}

func TestFilesBudgetTokens(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	proj := write("proj.json", `{"orchestration":{"budget_tokens":3000}}`)
	user := write("user.json", `{"orchestration":{"budget_tokens":9000}}`)
	bad := write("bad.json", `{"orchestration":{"budget_tokens":"x"}}`)
	notObj := write("obj.json", `{"orchestration":5}`)
	none := write("none.json", `{"orchestration":{"workers":"read"}}`)
	cases := []struct {
		name  string
		loc   Locations
		want  int64
		notes int
	}{
		{"project wins", Locations{Project: proj, User: user}, 3000, 0},
		{"user when project absent", Locations{Project: filepath.Join(dir, "missing"), User: user}, 9000, 0},
		{"invalid skipped, next used", Locations{Project: bad, User: user}, 9000, 1},
		{"not an object", Locations{Project: notObj}, 0, 1},
		{"key absent", Locations{Project: none}, 0, 0},
		{"no files", Locations{}, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, notes := FilesBudgetTokens(c.loc)
			if n != c.want || len(notes) != c.notes {
				t.Fatalf("got %d,%v want %d,%d notes", n, notes, c.want, c.notes)
			}
		})
	}
}
