// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

// TestClassify_MechanicalSignals pins the mechanical-edit and command signals
// added from the 1000-prompt exploration set. Positive cases must reach a cheap
// class; the negative cases are look-alikes that must NOT, because the same
// words appear in real analysis or multi-step work.
func TestClassify_MechanicalSignals(t *testing.T) {
	cheap := func(c Complexity) bool { return c == Trivial || c == Simple }
	tests := []struct {
		name   string
		prompt string
		cheap  bool
	}{
		// positives, English
		{"delete commented block", "Delete the commented-out code block at the bottom of notebooks/feature_scratch.py", true},
		{"set default value", "Change the default batch size in config/train.yaml from 32 to 64.", true},
		{"bump dependency", "Bump vitest to 2.1.8 in package.json and run the suite.", true},
		{"gitignore entry", "Add .DS_Store and dist/ to .gitignore", true},
		{"run formatter", "cargo fmt && cargo clippy on the whole workspace", true},
		{"run tests short", "run the tests", true},
		{"list files", "List the files changed in the last commit touching src/payments/.", true},
		// positives, PT-BR
		{"pt remove debugger", "Remova o debugger que sobrou em src/checkout/Cart.tsx", true},
		{"pt change port", "Troque a porta padrão do servidor de 3000 para 8080 no config.ts", true},
		{"pt short explain", "Explica rapidinho o que é um índice parcial no Postgres.", true},
		{"pt difference", "qual a diferença entre DELETE e TRUNCATE?", true},
		// look-alikes that must not become cheap
		{"pt explain is an analysis", "Explique o fluxo completo de autenticação deste repositório (login, refresh, logout, middleware) e liste pontos frágeis de segurança que você encontrar.", false},
		{"run then analyse", "Run the test suite for the infra/ Terratest folder and summarise failures grouped by root cause.", false},
		{"change default across services", "Change the default retry policy across all payment services and make sure idempotency keys survive the retry", false},
	}
	for _, tt := range tests {
		got := Classify(tt.prompt).Complexity
		if cheap(got) != tt.cheap {
			t.Errorf("%s: complexity %s, want cheap=%v\n   prompt: %.100s", tt.name, got, tt.cheap, tt.prompt)
		}
	}
}
