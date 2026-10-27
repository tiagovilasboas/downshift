// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"testing"
)

// These independent acceptance examples exercise safety at Route and its
// capability plan, rather than merely mirroring the language detector.
// Risk floors may still deterministically lift small baselines; Codex effort
// is an independent capability. This test proves that frontier is preserved.
func TestMultilangAcceptance_UnsupportedLanguageIsNeverConfident(t *testing.T) {
	prompts := []string{
		"rename el campo que contiene las contraseñas de clientes",
		"format le module qui conserve les secrets du client",
		"rename die Variable in der Zahlungsabwicklung",
		"format 用户密码存储配置",
		"rename パスワード保存関数",
		"format пользовательские пароли",
		"rename 사용자 비밀번호 필드",
		"format كلمة مرور المستخدم",
	}
	for _, prompt := range prompts {
		if d := classifiedRoute(prompt, "claude-code", ""); d.Confident || !d.UnsupportedLanguage {
			t.Errorf("%q: confident=%v unsupported=%v, want an unsupported, unconfident read", prompt, d.Confident, d.UnsupportedLanguage)
		}
	}
}

func TestMultilangAcceptance_PortugueseRiskFloorAcrossHarnesses(t *testing.T) {
	prompts := []string{
		"Renomeie a coluna que guarda dados pessoais dos clientes",
		"Corrija o typo na verificação de permissão do administrador",
		"Formate o código que detecta injeção de SQL",
		"Adicione um comentário na tabela de privilégios",
	}
	for _, h := range core.AllHarnessCaps() {
		for _, prompt := range prompts {
			d := core.Route(prompt, h.Harness, catID(h.Harness, core.TierFrontier), cat)
			if d.Tier < core.TierMid {
				t.Errorf("%s %q below risk floor: %+v", h.Harness, prompt, d)
			}
		}
	}
}
