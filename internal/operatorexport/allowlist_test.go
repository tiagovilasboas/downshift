// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package operatorexport

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestCheckRejectsLeakyFields is the sensor: every leaky export below carries a
// string the allowlist does not allow, so Check must fail on each. If Check
// stops failing on one of these, a private string could leave the machine.
func TestCheckRejectsLeakyFields(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string // a substring the error must mention
	}{
		{
			name: "file path under a new field",
			json: `{"schema":1,"session_path":"/Users/alice/.harness-downshift/events.jsonl"}`,
			want: "session_path",
		},
		{
			name: "session id under a new field",
			json: `{"schema":1,"session_id":"a1b2c3d4-5566-7788-99aa-bbccddeeff00"}`,
			want: "session_id",
		},
		{
			name: "model free text under a new field",
			json: `{"schema":1,"model":"claude-opus-4-5-20260101"}`,
			want: "model",
		},
		{
			name: "hostname under a new field",
			json: `{"schema":1,"host":"tiagos-macbook.local"}`,
			want: "host",
		},
		{
			name: "username under a new field",
			json: `{"schema":1,"user":"tiago.boas"}`,
			want: "user",
		},
		{
			name: "prompt text under note-looking field",
			json: `{"schema":1,"prompt":"implement the operator export command"}`,
			want: "prompt",
		},
		{
			name: "unknown harness value on an allowed field",
			json: `{"schema":1,"harnesses":[{"harness":"my-secret-internal-tool"}]}`,
			want: "value allowlist",
		},
		{
			name: "tampered note value on the allowed note field",
			json: `{"schema":1,"note":"ran from /home/alice as alice@host"}`,
			want: "value allowlist",
		},
		{
			name: "path hidden in a nested harness row under a stray key",
			json: `{"schema":1,"harnesses":[{"harness":"codex","cwd":"/Users/alice/project"}]}`,
			want: "cwd",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckJSON([]byte(tc.json))
			if err == nil {
				t.Fatalf("Check passed a leaky export; it must fail on %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name the offending part %q", err.Error(), tc.want)
			}
		})
	}
}

// TestCheckAllowsCleanExport makes sure the allowlist is not so strict it
// rejects a well-formed export: a known harness name and the fixed note pass.
func TestCheckAllowsCleanExport(t *testing.T) {
	clean := Export{
		Schema:      SchemaVersion,
		WindowDays:  7,
		GeneratedAt: "2026-11-01T00:00:00Z",
		Routed:      Group{Spawns: 10, Ended: 9, Completed: 8, Reworked: 1},
		Tokens:      Tokens{RoutedBillable: 1000, CompactorAvoided: 50},
		Harnesses: []HarnessRow{
			{Harness: "claude-code", SessionBillable: 900, BelowSessionBillable: 300, CompactorAvoided: 50},
			{Harness: "codex"},
			// The token aggregation uses this sentinel for exec events with no
			// harness; it is a fixed label, not an identity, so it must pass.
			{Harness: "unknown", CompactorAvoided: 10},
		},
		Note: Note,
	}
	if err := Check(clean); err != nil {
		t.Fatalf("Check rejected a clean export: %v", err)
	}
}

// TestEveryKnownHarnessIsAllowed guards against an adapter name that Build
// could emit but the value allowlist would reject.
func TestEveryKnownHarnessIsAllowed(t *testing.T) {
	for _, h := range knownHarnessNames() {
		if !knownHarnessValue(h) {
			t.Errorf("known harness %q is not on the value allowlist", h)
		}
	}
}

// TestCheckRejectsKeysAndMisboundValues covers what a string-only sensor
// misses: a private key with a numeric value, a timestamp field carrying a
// path, and a string allowed on one field but put on another.
func TestCheckRejectsKeysAndMisboundValues(t *testing.T) {
	cases := []struct {
		name, json, want string
	}{
		{"map keyed by repo", `{"by_repo":{"/Users/alice/acme-secret":12,"tiagos-macbook.local":3}}`, "by_repo"},
		{"map keyed by model", `{"per_model":{"claude-opus-4-5-20260101":1000}}`, "per_model"},
		{"private key inside tokens", `{"tokens":{"/Users/alice":1}}`, "/Users/alice"},
		{"generated_at nested in a harness row", `{"harnesses":[{"harness":"codex","generated_at":"/Users/alice/project"}]}`, "generated_at"},
		{"generated_at not a timestamp", `{"generated_at":"alice@host:/Users/alice"}`, "value allowlist"},
		{"note holding a harness name", `{"note":"codex"}`, "value allowlist"},
		{"harness holding the note", `{"harnesses":[{"harness":` + quote(Note) + `}]}`, "value allowlist"},
		{"string where a count belongs", `{"routed":{"spawns":"/Users/alice"}}`, "spawns"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckJSON([]byte(tc.json))
			if err == nil {
				t.Fatalf("Check passed %s", tc.json)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name %q", err.Error(), tc.want)
			}
		})
	}
}

// TestExportSchemaIsClosed is the struct sensor: Export must derive a schema,
// and a field the allowlist has no rule for must make derivation fail, even
// when the field would be empty (omitempty) in every real export.
func TestExportSchemaIsClosed(t *testing.T) {
	if _, err := schemaOf(reflect.TypeOf(Export{}), ""); err != nil {
		t.Fatalf("Export schema: %v", err)
	}
	type withMap struct {
		PerModel map[string]int64 `json:"per_model,omitempty"`
	}
	type withAny struct {
		Extra any `json:"extra,omitempty"`
	}
	type withString struct {
		Cwd string `json:"cwd,omitempty"`
	}
	type withNestedString struct {
		Rows []struct {
			Host string `json:"host"`
		} `json:"rows"`
	}
	for name, v := range map[string]any{"map": withMap{}, "interface": withAny{}, "string": withString{}, "nested string": withNestedString{}} {
		if _, err := schemaOf(reflect.TypeOf(v), ""); err == nil {
			t.Errorf("%s field derived a schema; it must be refused", name)
		}
	}
}

func quote(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}
