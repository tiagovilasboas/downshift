// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import "testing"

// Class routing is on by default, a file turns it off with routing.by_class,
// and the project's setting wins over the user's.
func TestByClass(t *testing.T) {
	cases := []struct {
		name          string
		project, user string
		want          bool
	}{
		{"no files: on", "", "", true},
		{"a routing section without the key: on", `{"version":1,"routing":{"critical_threshold":80}}`, "", true},
		{"project off", `{"version":1,"routing":{"by_class":false}}`, "", false},
		{"user off", "", `{"version":1,"routing":{"by_class":false}}`, false},
		{"project on beats user off", `{"version":1,"routing":{"by_class":true}}`, `{"version":1,"routing":{"by_class":false}}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc := Locations{}
			if c.project != "" {
				loc.Project = projectFile(t, c.project)
			}
			if c.user != "" {
				loc.User = userFile(t, c.user)
			}
			p, err := Load(loc)
			if err != nil {
				t.Fatal(err)
			}
			if p.ClassRouting() != c.want {
				t.Fatalf("ClassRouting = %v, want %v", p.ClassRouting(), c.want)
			}
		})
	}
	if !(*Policy)(nil).ClassRouting() {
		t.Fatal("a nil policy is the default policy: class routing must be on")
	}
}

func TestByClassRefusesANonBool(t *testing.T) {
	if _, err := Load(Locations{Project: projectFile(t, `{"version":1,"routing":{"by_class":"yes"}}`)}); err == nil {
		t.Fatal("by_class must be a boolean")
	}
}

// routing.raise_complex is off by default and on only where a file says so.
func TestRaiseComplex(t *testing.T) {
	p, err := Load(Locations{})
	if err != nil || p.ClassRaise() {
		t.Fatalf("default: ClassRaise = %v (err %v), want false", p.ClassRaise(), err)
	}
	p, err = Load(Locations{Project: projectFile(t, `{"version":1,"routing":{"raise_complex":true}}`)})
	if err != nil || !p.ClassRaise() {
		t.Fatalf("project on: ClassRaise = %v (err %v), want true", p.ClassRaise(), err)
	}
	if (*Policy)(nil).ClassRaise() {
		t.Fatal("a nil policy must not raise")
	}
}
