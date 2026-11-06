// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package surface

import "testing"

func TestStyled(t *testing.T) {
	env := func(kv map[string]string) func(string) string {
		return func(k string) string { return kv[k] }
	}
	tty := Facts{Args: []string{"report"}, StdinTTY: true, StdoutTTY: true}
	with := func(mod func(*Facts)) Facts { f := tty; mod(&f); return f }
	cases := []struct {
		name string
		f    Facts
		want bool
	}{
		{"terminal", tty, true},
		{"days flag", with(func(f *Facts) { f.Args = []string{"report", "--days=7"} }), true},
		{"no color still styled", with(func(f *Facts) { f.Getenv = env(map[string]string{"NO_COLOR": "1"}) }), true},
		{"stdout piped", with(func(f *Facts) { f.StdoutTTY = false }), false},
		{"stdin piped", with(func(f *Facts) { f.StdinTTY = false }), false},
		{"plain flag", with(func(f *Facts) { f.Args = []string{"report", "--plain"} }), false},
		{"plain env", with(func(f *Facts) { f.Getenv = env(map[string]string{"DOWNSHIFT_PLAIN": "1"}) }), false},
		{"text", with(func(f *Facts) { f.Args = []string{"report", "--text"} }), false},
		{"json", with(func(f *Facts) { f.Args = []string{"report", "--json"} }), false},
		{"markdown", with(func(f *Facts) { f.Args = []string{"report", "--markdown"} }), false},
		{"term dumb", with(func(f *Facts) { f.Getenv = env(map[string]string{"TERM": "dumb"}) }), false},
		{"ci", with(func(f *Facts) { f.Getenv = env(map[string]string{"CI": "true"}) }), false},
		{"agent shell", with(func(f *Facts) { f.Getenv = env(map[string]string{"CLAUDE_CODE_SESSION_ID": "x"}) }), false},
	}
	for _, c := range cases {
		if got := Styled(c.f); got != c.want {
			t.Errorf("%s: Styled = %v, want %v", c.name, got, c.want)
		}
	}
}
