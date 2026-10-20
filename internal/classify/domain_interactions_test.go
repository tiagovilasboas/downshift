// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package classify

import "testing"

func TestNativeInteractionsRewardIndependentDomainsOnce(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "1")
	var e domainEvidence
	e.add("backend", Medium, 2)
	e.add("backend", Medium, 2)             // Same specialty: capped at 3.
	e.add("data", Medium, 2)                // Independent corroboration: +1.
	scores := map[Complexity]int{Medium: 4} // Generic evidence stays additive.
	e.apply(scores)
	if scores[Medium] != 10 { // 4 generic + 3 backend + 2 data + 1 agreement.
		t.Fatalf("expected 10, got %+v", scores)
	}
	e.add("observability", Medium, 2)
	scores = map[Complexity]int{}
	e.apply(scores)
	if scores[Medium] != 8 { // 3 + 2 + 2 + 1, not a quadratic growth.
		t.Fatalf("third agreeing domain must not stack pair bonuses: %+v", scores)
	}
}

type interactionTestVote struct {
	domain string
	class  Complexity
	weight int
}

func TestNativeInteractionsRequireSeparateDomainsAndMatchingTiers(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "1")
	cases := []struct {
		name  string
		rules []interactionTestVote
		wantM int
		wantC int
	}{
		{"one domain repeated", []interactionTestVote{{"backend", Medium, 2}, {"backend", Medium, 2}}, 3, 0},
		{"opposing tiers", []interactionTestVote{{"backend", Medium, 2}, {"devops", Complex, 3}}, 2, 3},
		{"two complex domains", []interactionTestVote{{"devops", Complex, 3}, {"data", Complex, 3}}, 0, 7},
		{"simple work unaffected", []interactionTestVote{{"backend", Simple, 2}, {"data", Simple, 2}}, 0, 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var d domainEvidence
			for _, x := range tt.rules {
				d.add(x.domain, x.class, x.weight)
			}
			scores := make(map[Complexity]int)
			d.apply(scores)
			if scores[Medium] != tt.wantM || scores[Complex] != tt.wantC {
				t.Fatalf("unexpected evidence score: %+v", scores)
			}
			if tt.name == "simple work unaffected" && scores[Simple] != 4 {
				t.Fatalf("simple work must not gain pair bonus: %+v", scores)
			}
		})
	}
}

func TestNativeInteractionsCanBeComparedWithoutDisablingVocabulary(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "1")
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	cases := []struct {
		name  string
		tier  Complexity
		en    string
		pt    string
		mixed string
	}{
		{"fullstack and backend", Medium,
			"Implement API error handling in UI and implement idempotency key validation",
			"Implemente tratamento de erros de API na interface e implemente validação de chave de idempotência",
			"Implement API error handling in UI e implemente validação de chave de idempotência"},
		{"devops and data", Complex,
			"Design service mesh mTLS rollout and design CDC across multiple databases",
			"Projete implantação mTLS da malha de serviços e projete CDC entre múltiplos bancos de dados",
			"Design service mesh mTLS rollout e projete CDC entre múltiplos bancos de dados"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			for _, p := range []struct{ lang, text string }{
				{"en", tt.en}, {"pt-BR", tt.pt}, {"mixed", tt.mixed},
			} {
				t.Run(p.lang, func(t *testing.T) {
					t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "0")
					without := Classify(p.text)
					t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "1")
					with := Classify(p.text)
					if with.Scores[tt.tier] != without.Scores[tt.tier]+1 {
						t.Errorf("expected one native corroboration point: without=%+v with=%+v", without.Scores, with.Scores)
					}
					if with.Complexity < without.Complexity {
						t.Errorf("corroboration reduced class: without=%v with=%v", without.Complexity, with.Complexity)
					}
				})
			}
		})
	}
}

func TestNativeInteractionsDoNotChangeExistingRiskFloor(t *testing.T) {
	t.Setenv("DOWNSHIFT_DOMAIN_SIGNALS", "1")
	t.Setenv("DOWNSHIFT_DOMAIN_DEDUP", "1")
	for _, text := range []string{
		"Fix the authorization bypass in the access control policy",
		"Corrija o bypass de autorização no controle de acesso",
	} {
		t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "0")
		before := Classify(text)
		t.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "1")
		after := Classify(text)
		if before.RiskFloor != after.RiskFloor || before.RiskAbstain != after.RiskAbstain {
			t.Errorf("risk guards changed for %q: before=%+v after=%+v", text, before, after)
		}
	}
}

func BenchmarkNativeInteractions(b *testing.B) {
	b.Setenv("DOWNSHIFT_DOMAIN_INTERACTIONS", "1")
	prompt := "Implement API error handling in UI and implement idempotency key validation"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Classify(prompt)
	}
}
