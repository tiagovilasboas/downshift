// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMaxWorkersPerSession(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name string
		loc  Locations
		want int
	}{
		{"none", Locations{}, 32},
		{"project lowers", Locations{Project: write("p.json", `{"orchestration":{"max_workers_per_session":5}}`), User: write("u.json", `{"orchestration":{"max_workers_per_session":9}}`)}, 5},
		{"project cannot raise", Locations{Project: write("pr.json", `{"orchestration":{"max_workers_per_session":10000}}`)}, 32},
		{"project cannot remove", Locations{Project: write("p0.json", `{"orchestration":{"max_workers_per_session":0}}`)}, 32},
		{"project lowers no cap", Locations{Project: write("p3.json", `{"orchestration":{"max_workers_per_session":3}}`), User: write("u0.json", `{"orchestration":{"max_workers_per_session":0}}`)}, 3},
		{"zero is no cap", Locations{User: write("z.json", `{"orchestration":{"max_workers_per_session":0}}`)}, 0},
		{"negative skipped", Locations{Project: write("n.json", `{"orchestration":{"max_workers_per_session":-1}}`), User: write("u2.json", `{"orchestration":{"max_workers_per_session":7}}`)}, 7},
		{"garbage", Locations{Project: write("g.json", `{"orchestration":{"max_workers_per_session":"x"}}`)}, 32},
	}
	for _, c := range cases {
		if got := MaxWorkersPerSession(c.loc); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}
