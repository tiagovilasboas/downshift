// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// wrap builds a policy file whose routing section is body.
func wrap(body string) string {
	return `{"version":1,"routing":` + body + `}`
}

const oneProfile = `{"name":"reviewer","tier":"frontier","harness":"claude-code"}`

// Every refusal names its source, its field and the problem. A config error
// never produces a policy: the file is refused, not partly applied.
func TestConfigErrorsAreRefusedWithField(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		field string // the field the error names
		want  string // a substring of the problem
	}{
		{"threshold above 100", wrap(`{"critical_threshold":101}`), "routing.critical_threshold", "between 0 and 100"},
		{"threshold below 0", wrap(`{"critical_threshold":-1}`), "routing.critical_threshold", "between 0 and 100"},
		{"ttl zero", wrap(`{"quota_ttl_seconds":0}`), "routing.quota_ttl_seconds", "must be positive"},
		{"ttl negative", wrap(`{"quota_ttl_seconds":-300}`), "routing.quota_ttl_seconds", "must be positive"},
		{"ttl overflows", wrap(`{"quota_ttl_seconds":10000000000}`), "routing.quota_ttl_seconds", "too large"},
		{"both critical unknown", wrap(`{"both_critical":"wait"}`), "routing.both_critical", "ask, run-cheapest or block"},
		{"unknown routing key", wrap(`{"critical":80}`), "routing", "unknown field"},
		{"routing not an object", `{"version":1,"routing":3}`, "routing", "cannot unmarshal"},
		{"routing null", `{"version":1,"routing":null}`, "routing", "must be an object"},
		{"routing without version", `{"routing":{}}`, "version", "must be 1"},
		{"routing with version 2", `{"version":2,"routing":{}}`, "version", "must be 1"},
		{"file is not JSON", `{"version":1,`, "file", "not valid JSON"},
		{"profile name invalid", wrap(`{"profiles":[{"name":"Reviewer","tier":"frontier","harness":"claude-code"}]}`), "routing.profiles[0].name", "not a valid name"},
		{"profile duplicated", wrap(`{"profiles":[` + oneProfile + `,` + oneProfile + `]}`), "routing.profiles[1].name", "duplicate profile"},
		{"profile tier unknown", wrap(`{"profiles":[{"name":"p","tier":"strong","harness":"claude-code"}]}`), "routing.profiles[0].tier", "small, mid or frontier"},
		{"profile tier missing", wrap(`{"profiles":[{"name":"p","harness":"claude-code"}]}`), "routing.profiles[0].tier", "small, mid or frontier"},
		{"profile harness unknown", wrap(`{"profiles":[{"name":"p","tier":"mid","harness":"vscode"}]}`), "routing.profiles[0].harness", "unknown harness"},
		{"profile fallback invalid", wrap(`{"profiles":[{"name":"p","tier":"mid","harness":"codex","fallback":"Bad Name"}]}`), "routing.profiles[0].fallback", "not a valid profile name"},
		{"rule id invalid", wrap(`{"rules":[{"id":"Fix the auth bug","tier":"mid"}]}`), "routing.rules[0].id", "not a valid id"},
		{"rule id duplicated", wrap(`{"rules":[{"id":"x","tier":"mid"},{"id":"x","tier":"small"}]}`), "routing.rules[1].id", "duplicate rule"},
		{"rule kind unknown", wrap(`{"rules":[{"id":"x","kinds":["mechanical"],"tier":"mid"}]}`), "routing.rules[0].kinds[0]", "unknown kind"},
		{"rule effort unknown", wrap(`{"rules":[{"id":"x","efforts":["max"],"tier":"mid"}]}`), "routing.rules[0].efforts[0]", "unknown effort class"},
		{"rule tier unknown", wrap(`{"rules":[{"id":"x","tier":"huge"}]}`), "routing.rules[0].tier", "small, mid or frontier"},
		{"rule names profile and tier", wrap(`{"profiles":[` + oneProfile + `],"rules":[{"id":"x","profile":"reviewer","tier":"small"}]}`), "routing.rules[0]", "not both"},
		{"rule names neither", wrap(`{"rules":[{"id":"x"}]}`), "routing.rules[0]", "set one of profile or tier"},
		{"rule profile unknown", wrap(`{"rules":[{"id":"x","profile":"ghost"}]}`), "routing.rules[x].profile", `unknown profile "ghost"`},
		{"gate window unknown", wrap(`{"rules":[{"id":"x","tier":"mid","gates":[{"harness":"claude-code","window":"opus","below":80}]}]}`), "routing.rules[0].gates[0].window", "short, week or model"},
		{"gate below above 100", wrap(`{"rules":[{"id":"x","tier":"mid","gates":[{"harness":"claude-code","window":"week","below":120}]}]}`), "routing.rules[0].gates[0].below", "between 0 and 100"},
		{"gate without below", wrap(`{"rules":[{"id":"x","tier":"mid","gates":[{"harness":"claude-code","window":"week"}]}]}`), "routing.rules[0].gates[0].below", "required"},
		{"gate harness unknown", wrap(`{"rules":[{"id":"x","tier":"mid","gates":[{"harness":"vscode","window":"week","below":80}]}]}`), "routing.rules[0].gates[0].harness", "unknown harness"},
		{"fallback names no profile", wrap(`{"profiles":[{"name":"a","tier":"mid","harness":"codex","fallback":"ghost"}]}`), "routing.profiles[a].fallback", `unknown profile "ghost"`},
		{"fallback names itself", wrap(`{"profiles":[{"name":"a","tier":"mid","harness":"codex","fallback":"a"}]}`), "routing.profiles[a].fallback", "cannot fall back to itself"},
		{"fallback widens the tier", wrap(`{"profiles":[{"name":"a","tier":"small","harness":"codex","fallback":"b"},{"name":"b","tier":"frontier","harness":"codex"}]}`), "routing.profiles[a].fallback", "never widens"},
		{"fallback chain", wrap(`{"profiles":[{"name":"a","tier":"mid","harness":"codex","fallback":"b"},{"name":"b","tier":"small","harness":"codex","fallback":"c"},{"name":"c","tier":"small","harness":"codex"}]}`), "routing.profiles[a].fallback", "chains are not supported"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := projectFile(t, c.body)
			p, err := Load(Locations{Project: path})
			if err == nil {
				t.Fatalf("accepted, want refusal: %s", c.body)
			}
			if p != nil {
				t.Fatalf("refusal also returned a policy: %+v", p)
			}
			var ce *ConfigError
			if !errors.As(err, &ce) {
				t.Fatalf("error %T is not a *ConfigError", err)
			}
			if ce.Source != path {
				t.Errorf("Source = %q, want the file %q", ce.Source, path)
			}
			if ce.Field != c.field {
				t.Errorf("Field = %q, want %q (error: %v)", ce.Field, c.field, err)
			}
			if !strings.Contains(ce.Problem, c.want) && !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

// An unknown profile in a rule is also refused with its reason, so a note can
// carry it.
func TestUnknownProfileCarriesReason(t *testing.T) {
	_, err := Load(Locations{Project: projectFile(t, wrap(`{"rules":[{"id":"x","profile":"ghost"}]}`))})
	var ce *ConfigError
	if !errors.As(err, &ce) || ce.Reason != contract.ReasonUnknownProfile {
		t.Fatalf("reason = %+v, want unknown_profile", err)
	}
}

// Boundary values are accepted: a threshold of 0 and 100, a gate of 0.
func TestBoundaryValuesAreAccepted(t *testing.T) {
	path := projectFile(t, wrap(`{"critical_threshold":100,"rules":[{"id":"x","tier":"mid","gates":[{"harness":"codex","window":"short","below":0}]}]}`))
	p, err := Load(Locations{Project: path})
	if err != nil {
		t.Fatalf("boundary values refused: %v", err)
	}
	if p.CriticalThreshold != 100 || p.Rules[0].Gates[0].Below != 0 {
		t.Fatalf("boundaries = %v, %v", p.CriticalThreshold, p.Rules[0].Gates[0].Below)
	}
}
