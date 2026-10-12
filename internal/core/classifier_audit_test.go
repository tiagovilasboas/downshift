// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Regression cases from the routing audit (labs research/routing-audit.md):
// a single keyword mention scored twice, architectural words in a prose-only
// edit bought a frontier model, bare "auth" and PT-BR secret words did not
// raise the risk floor, and system-design prompts with no keyword stayed mid.

// One mention of a stem counts once, EN and PT-BR alike.
func TestClassify_SingleMentionScoresOnce(t *testing.T) {
	for _, p := range []string{
		"migrate",
		"migration",
		"migrações",
		"migre",
		"architect",
		"rearchitect",
		"re-architect",
	} {
		if got := core.Classify(p).Scores[core.Complex]; got != 3 {
			t.Errorf("%q: Complex score %d, want 3 (one mention, one hit)", p, got)
		}
	}
}

// A short edit whose target is documentation or comments is not
// architecture work, whatever topic word it names.
func TestClassify_DocEditIsNotComplex(t *testing.T) {
	for _, c := range []struct {
		prompt string
		want   core.Complexity // -1: anything below Complex
	}{
		{"fix a typo in the migrations README", core.Trivial},
		{"rename cron to scheduler in the docs", core.Trivial},
		{"renomeie cron para scheduler na documentação", core.Trivial},
		{"add a graph of weekly usage to the README", -1},
		{"adicione um gráfico de uso semanal ao README", -1},
		{"corrija um erro de digitação no README das migrações", -1},
		{"update the comment on the distributed lock helper", -1},
	} {
		cls := core.Classify(c.prompt)
		if cls.Complexity == core.Complex || (c.want >= 0 && cls.Complexity != c.want) {
			t.Errorf("%q: Classify=%s scores=%v, want %v", c.prompt, cls.Complexity, cls.Scores, c.want)
		}
		if d := core.Route(c.prompt, "claude-code", ""); d.Tier == core.TierFrontier {
			t.Errorf("%q: Route tier frontier (complexity %s)", c.prompt, d.Complexity)
		}
	}
}

// The prose damper must not touch real architecture work, a second clause
// carrying real work, or a document that is only referenced.
func TestClassify_DocDamperKeepsRealWork(t *testing.T) {
	for _, p := range []string{
		"migrate the database schema across services",
		"migre o esquema do banco de dados entre os serviços",
		"migrate the users table to the new schema",
		"rename cron to scheduler in the docs and migrate the scheduler to kubernetes",
		"fix the race condition described in the comments",
		"migrate the docs site to a new architecture",
	} {
		if cls := core.Classify(p); cls.Complexity != core.Complex {
			t.Errorf("%q: Classify=%s scores=%v, want COMPLEX", p, cls.Complexity, cls.Scores)
		}
	}
}

// Designing or building a whole system is frontier work even without an
// architecture keyword. These are the labs seed prompts that failed the
// frontier-to-mid gate; the look-alikes below must not move.
func TestClassify_SystemDesignIsComplex(t *testing.T) {
	for _, p := range []string{
		"design a real-time collaborative editing system with operational transformation",
		"design and build a financial reconciliation engine handling multi-currency",
		"design and build a recommendation engine using collaborative filtering",
		"build real-time collaboration platform with operational transformation",
		"design database sharding strategy for 100M+ record tables",
		"design consensus algorithm for cross-datacenter state machine replication",
		"projete a estratégia de sharding do banco de pedidos",
	} {
		if d := core.Route(p, "claude-code", ""); d.Tier != core.TierFrontier {
			t.Errorf("%q: tier %s (complexity %s), want frontier", p, d.Tier, d.Complexity)
		}
	}
	for _, p := range []string{
		"replicate the bug locally and add a failing test",
		"shard the Playwright suite across 4 runners",
		"design the login form",
		"build the docs site",
	} {
		if cls := core.Classify(p); cls.Complexity == core.Complex {
			t.Errorf("%q: Classify=%s scores=%v, want below COMPLEX", p, cls.Complexity, cls.Scores)
		}
	}
}

// Bare auth and PT-BR secret/auth words raise the risk floor, so a rename
// there is never a confident small-tier downgrade. Risk stays separate from
// complexity: the Complex score is untouched.
func TestRiskFloor_AuthAndPortuguese(t *testing.T) {
	for _, c := range []struct {
		prompt string
		cat    string
	}{
		{"rename the auth middleware to guard", "authz"},
		{"rename authMiddleware to guardMiddleware", "authz"},
		{"renomeie o middleware de autenticação para guard", "authz"},
		{"renomeie a função de autorização para checkAccess", "authz"},
		{"renomeie a variável senha para pwd", "secret"},
		{"renomeie a credencial do banco para dbCred", "secret"},
		{"rename the credenciais struct to creds", "secret"},
	} {
		cls := core.Classify(c.prompt)
		if !cls.RiskFloor || cls.Confident || cls.Complexity.Tier() == core.TierSmall {
			t.Errorf("%q: complexity=%s confident=%v riskFloor=%v, want risk-floored mid", c.prompt, cls.Complexity, cls.Confident, cls.RiskFloor)
		}
		if cats := core.RiskCategories(c.prompt); !slices.Contains(cats, c.cat) {
			t.Errorf("%q: risk %v, want %s", c.prompt, cats, c.cat)
		}
		if cls.Scores[core.Complex] != 0 {
			t.Errorf("%q: risk leaked into the Complex score (%d)", c.prompt, cls.Scores[core.Complex])
		}
		if d := core.Route(c.prompt, "claude-code", ""); d.Tier == core.TierSmall {
			t.Errorf("Route(%q).Tier = small", c.prompt)
		}
	}
	// "auth" inside a path, file or hyphenated name is a location.
	for _, p := range []string{
		"rename the variable in auth.ts",
		"format auth/handlers.go",
		"Delete the commented-out tests in tests/test_legacy_auth.py",
		"show the last 50 logs of pod auth-7d9f",
		"add an author field to the Post struct",
	} {
		if cats := core.RiskCategories(p); len(cats) > 0 {
			t.Errorf("%q matched risk categories %v", p, cats)
		}
	}
}
