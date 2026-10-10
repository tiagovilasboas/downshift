// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package hookport_test

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookport"
)

func TestObserveNilFuncsAreUnobserved(t *testing.T) {
	p := hookport.Port{ID: "cursor"}
	raw := []byte(`{}`)

	honor := hookport.ObserveHonor(p, raw, "test", nil)
	if honor.Observed {
		t.Fatal("nil Honor was observed")
	}
	if honor.Stdout != nil {
		t.Fatalf("nil Honor stdout = %#v, want nil", honor.Stdout)
	}
	if honor.Note != "" {
		t.Fatalf("nil Honor note = %q, want empty", honor.Note)
	}

	usage := hookport.ObserveUsage(p, raw, "test", nil)
	if usage.Observed {
		t.Fatal("nil Usage was observed")
	}
	if usage.Stdout != nil {
		t.Fatalf("nil Usage stdout = %#v, want nil", usage.Stdout)
	}
	if usage.Note != "" {
		t.Fatalf("nil Usage note = %q, want empty", usage.Note)
	}
}

func TestObserveHonorPassesNote(t *testing.T) {
	const wantNote = "honored fixture"
	p := hookport.Port{
		ID: "fixture",
		Honor: func(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
			return map[string]string{"model": "fixture-model"}, wantNote
		},
	}

	obs := hookport.ObserveHonor(p, []byte(`{"child":true}`), "v-test", nil)
	if !obs.Observed {
		t.Fatal("set Honor was not observed")
	}
	if obs.Note != wantNote {
		t.Fatalf("note = %q, want %q", obs.Note, wantNote)
	}
	got, ok := obs.Stdout.(map[string]string)
	if !ok || got["model"] != "fixture-model" {
		t.Fatalf("stdout = %#v, want model fixture-model", obs.Stdout)
	}
}

func TestObserveHonorNilStdoutMarshalsEmptyObject(t *testing.T) {
	p := hookport.Port{
		ID: "fixture",
		Honor: func(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
			return nil, "nil stdout"
		},
	}

	obs := hookport.ObserveHonor(p, []byte(`{}`), "v-test", nil)
	if !obs.Observed {
		t.Fatal("Honor that returned nil stdout was not observed")
	}
	if obs.Note != "nil stdout" {
		t.Fatalf("note = %q, want %q", obs.Note, "nil stdout")
	}
	encoded, err := json.Marshal(obs.Stdout)
	if err != nil {
		t.Fatalf("json.Marshal(stdout) error = %v", err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("json.Marshal(stdout) = %s, want {}", encoded)
	}
}

func TestObserveHonorEmptyResultIsObserved(t *testing.T) {
	p := hookport.Port{
		ID: "fixture",
		Honor: func(raw []byte, binaryVersion string, r core.Resolver) (any, string) {
			return nil, ""
		},
	}

	obs := hookport.ObserveHonor(p, []byte(`{}`), "v-test", nil)
	if !obs.Observed {
		t.Fatal("empty Honor result was not observed")
	}
	if obs.Note != "" {
		t.Fatalf("note = %q, want empty", obs.Note)
	}
	encoded, err := json.Marshal(obs.Stdout)
	if err != nil {
		t.Fatalf("json.Marshal(stdout) error = %v", err)
	}
	if string(encoded) != "{}" {
		t.Fatalf("json.Marshal(stdout) = %s, want {}", encoded)
	}
}

func TestObserveUsagePassesNoteWithoutStdout(t *testing.T) {
	const wantNote = "usage fixture"
	p := hookport.Port{
		ID: "fixture",
		Usage: func(raw []byte, binaryVersion string, r core.Resolver) string {
			return wantNote
		},
	}

	obs := hookport.ObserveUsage(p, []byte(`{"tokens":1}`), "v-test", nil)
	if !obs.Observed {
		t.Fatal("set Usage was not observed")
	}
	if obs.Note != wantNote {
		t.Fatalf("note = %q, want %q", obs.Note, wantNote)
	}
	if obs.Stdout != nil {
		t.Fatalf("ObserveUsage invented stdout %#v", obs.Stdout)
	}
}

func TestObserveUsageEmptyResultIsObserved(t *testing.T) {
	p := hookport.Port{
		ID: "fixture",
		Usage: func(raw []byte, binaryVersion string, r core.Resolver) string {
			return ""
		},
	}

	obs := hookport.ObserveUsage(p, []byte(`{}`), "v-test", nil)
	if !obs.Observed {
		t.Fatal("empty Usage result was not observed")
	}
	if obs.Note != "" {
		t.Fatalf("note = %q, want empty", obs.Note)
	}
	if obs.Stdout != nil {
		t.Fatalf("ObserveUsage invented stdout %#v", obs.Stdout)
	}
}
