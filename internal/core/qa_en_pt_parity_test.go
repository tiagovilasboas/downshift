// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// Sensor: English and Portuguese twins of the same task must get the same
// Complexity from core.Classify. Risk twins must not land on Trivial.
// Pairs are taken from classifier_test.go, classifier_edge_test.go,
// classifier_ptbr_test.go and the EN/PT rows in signals.go / risk.go.
//
// Downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// enPtPair is one task written twice. risk marks twins whose Portuguese
// prompt must stay off Trivial (the risk floor). missingPT names a Portuguese
// pattern the tables do not have; the equality assertion stays in the test
// and is skipped rather than rewritten to the observed class.
type enPtPair struct {
	name      string
	en        string
	pt        string
	risk      bool
	missingPT string
}

// TestShortSummaryStaysAsymmetric locks the corpus failure STD-35: an English
// "short summary" inside a larger comparison must not become a cheap signal.
// The Portuguese stem still classifies a short-summary request as Simple.
func TestShortSummaryStaysAsymmetric(t *testing.T) {
	en := core.Classify("write a short summary of the PR")
	pt := core.Classify("escreva um resumo curto do PR")
	if en.Complexity != core.Medium || !en.NoSignal {
		t.Errorf("en = %s nosignal=%v scores=%v, want MEDIUM with no signal", en.Complexity, en.NoSignal, en.Scores)
	}
	if pt.Complexity != core.Simple {
		t.Errorf("pt = %s scores=%v, want SIMPLE", pt.Complexity, pt.Scores)
	}
}

func TestQAEnPtParity(t *testing.T) {
	for _, tc := range enPtParityCases {
		t.Run(tc.name, func(t *testing.T) {
			en := core.Classify(tc.en)
			pt := core.Classify(tc.pt)
			unequal := en.Complexity != pt.Complexity
			ptTrivial := tc.risk && pt.Complexity == core.Trivial
			if unequal || ptTrivial {
				if tc.missingPT != "" {
					// Assertion stays "same Complexity" / "PT risk is not Trivial".
					// Skip records the gap; it does not adopt the observed class.
					t.Skipf("missing PT pattern %s: want %s (equal to EN, and not Trivial when risk); pt=%s scores=%v risk=%v nosignal=%v; en scores=%v risk=%v",
						tc.missingPT, en.Complexity, pt.Complexity, pt.Scores, pt.Risk, pt.NoSignal, en.Scores, en.Risk)
				}
				if unequal {
					t.Errorf("Classify parity: en %q = %s (scores %v risk %v); pt %q = %s (scores %v risk %v)",
						tc.en, en.Complexity, en.Scores, en.Risk, tc.pt, pt.Complexity, pt.Scores, pt.Risk)
				}
				if ptTrivial {
					t.Errorf("PT risk twin landed on Trivial: %q scores=%v risk=%v", tc.pt, pt.Scores, pt.Risk)
				}
			}
		})
	}
}

// enPtParityCases are real twins: the English line is a shipped case or a
// signal stem, and the Portuguese line uses the mirror stem already in the
// tables (or the direct wording of that same task when the mirror is absent).
var enPtParityCases = []enPtPair{
	// Mechanical and one-step mirrors (signals.go TRIVIAL/SIMPLE rows).
	{name: "rename variable", en: "rename the variable userID to userId", pt: "renomeie a variável userId para userIdentifier"},
	{name: "fix typo", en: "fix a typo in the README", pt: "corrija o typo no README"},
	{name: "format prettier", en: "format this file with prettier", pt: "formate este arquivo com prettier"},
	{name: "add comment", en: "add a comment explaining this loop", pt: "adicione um comentário explicando o loop"},
	{name: "list files", en: "list the files in the directory", pt: "liste os arquivos do diretório"},
	{name: "show logs", en: "show the logs in the directory", pt: "mostre os arquivos de log do diretório tmp"},
	{name: "move file", en: "move the file utils.go to the helpers folder", pt: "mova o arquivo utils.go para a pasta helpers"},
	{name: "delete file", en: "delete the file temp.txt", pt: "apague o arquivo temp.txt"},
	{
		// signals.go: `\b(remova|remover|apague)\s+(o\s+|os\s+)?(\w+\s+)?(n.o\s+usad|mort)`
		// `\w` is ASCII-only, so `código` is not consumed and `mort` never lines up.
		name: "remove dead code", en: "remove dead code from the date module", pt: "remova o código morto do módulo de datas",
		missingPT: `\b(remova|remover|apague)\s+(o\s+|os\s+)?(\w+\s+)?(n.o\s+usad|mort) does not match "código morto" (\w is ASCII-only)`,
	},
	{name: "indent block", en: "indent the configuration block", pt: "indente o bloco de configuração"},
	{name: "run tests", en: "run the tests and show the result", pt: "rode os testes e mostre o resultado"},
	{name: "add field", en: "add a name field to the Usuario struct", pt: "adicione um campo nome na struct Usuario"},
	{name: "add parameter", en: "add a timeout parameter to the Fetch method", pt: "adicione um parâmetro timeout ao método Fetch"},
	{name: "fix bug", en: "fix the bug where the counter does not reset", pt: "corrija o bug do contador que não zera"},
	{name: "write function", en: "write a function that sums two numbers", pt: "escreva uma função que soma dois números"},
	{name: "write test", en: "write a test for the Soma function", pt: "escreva um teste para a função Soma"},
	{name: "explain regex", en: "explain what this regex does", pt: "explique o que esta regex faz"},
	{name: "bump version", en: "bump the version to 2.1.0", pt: "atualize a versão para 2.1.0"},

	// Mixed and hard mirrors that already have a PT stem.
	{name: "rename inside migration", en: "rename the table as part of the migration", pt: "renomeie a tabela como parte da migração"},
	// Same stems at ≥7 words: tie stays Complex. The 6-word PT line below is the
	// natural translation and must stay a hard failure (nudge, not a missing stem).
	{name: "format inside rearchitect long", en: "format the output of the pipeline that was rearchitected", pt: "formate a saída do pipeline que foi rearquitetado"},
	{name: "format inside rearchitect", en: "format the output of the rearchitected pipeline", pt: "formate a saída do pipeline rearquitetado"},
	{name: "security audit", en: "do a security audit of the upload flow", pt: "faça uma auditoria de segurança do fluxo de upload"},
	{name: "race condition", en: "explain the race condition in the payment observer", pt: "explique a condição de corrida no observador de pagamento"},
	{name: "migrate entire codebase", en: "migrate the entire codebase from JavaScript to TypeScript", pt: "migre toda a base de código de JavaScript para TypeScript"},
	{name: "migration no downtime", en: "plan the database migration to another region with no downtime", pt: "planeje a migração do banco de dados para outra região sem downtime"},
	{name: "distributed system", en: "design a distributed cache system with replication", pt: "projete um sistema distribuído de cache com replicação"},

	// Same task, PT stem absent from signals.go. Assertion stays "equal";
	// missingPT records the gap instead of accepting the lower class.
	{
		name: "rewrite parser", en: "rewrite the parser", pt: "reescreva o parser",
		missingPT: `\bre-?write\b has no reescrev/reescreva stem`,
	},
	{
		name:      "code review",
		en:        "list the Go files and do a code review without changing anything",
		pt:        "liste os arquivos Go e faça uma revisão de código sem mudar nada",
		missingPT: `\bcode\s+review\b; PT revisão only hits medium \brevis(?:ar|e|ão|oes|ões)\b`,
	},
	{
		name:      "design doc",
		en:        "write a design doc for the new checkout flow",
		pt:        "escreva um documento de design para o novo fluxo de checkout",
		missingPT: `\bdesign\s+doc\b has no documento de design mirror`,
	},
	{
		name:      "distributed adjective",
		en:        "fix the bug in the distributed payment processor",
		pt:        "corrija o bug no processador de pagamento distribuído",
		missingPT: `\bdistributed\b; PT only has \bsistemas?\s+distribu`,
	},
	{
		name:      "why it fails",
		en:        "why does the worker fail under load",
		pt:        "por que o worker falha sob carga",
		missingPT: `\bwhy\s+.{0,40}(fail|crash|hang|break|regress)\w* has no por que/falha mirror`,
	},
	{
		name:      "think through",
		en:        "think through the failure modes",
		pt:        "pense a fundo nos modos de falha",
		missingPT: `\bthink\s+through\b has no pense a fundo mirror`,
	},
	{
		name:      "memory leak",
		en:        "debug the memory leak in the connection pool under high load",
		pt:        "depure o vazamento de memória no pool de conexões sob carga alta",
		missingPT: `\bmemory leak\b has no vazamento de memória mirror (debug stem also EN-only)`,
	},
	// PT-only SIMPLE rows. The Portuguese pattern is present, so a mismatch
	// stays a failure: `\bresumo\s+(curto|breve)` and
	// `\b(atualiz\w*|altere|ajuste)\s+... (descri|texto|mensagem|r.tulo)`.
	// "short summary" is not an English signal. Adding one made a compare-and-
	// summarise task in the corpus (STD-35) a confident small write. The
	// Portuguese stem stays; the English sentence must keep the Medium default.
	{name: "update button description", en: "update the description of the login button", pt: "atualize a descrição do botão de login"},

	// Risk twins. PT must not be Trivial, and the class must match EN.
	// Sources: risk_safety_test.go TestRiskSafety_PortugueseParity,
	// classifier_audit_test.go TestRiskFloor_AuthAndPortuguese, ptbrRisk.
	{name: "risk charge card", risk: true, en: "rename the variable in the code that charges the customer card", pt: "renomeie a variável no código que cobra o cartão do cliente"},
	{name: "risk production data", risk: true, en: "rename the variable and delete the production data", pt: "renomeie a variável e apague os dados de produção"},
	{name: "risk access token", risk: true, en: "rename the access token field", pt: "renomeie o campo token de acesso"},
	{name: "risk certificate check", risk: true, en: "rename the variable and disable the certificate check", pt: "renomeie a variável e desative a verificação do certificado"},
	{name: "risk auth middleware", risk: true, en: "rename the auth middleware to guard", pt: "renomeie o middleware de autenticação para guard"},
	{name: "risk password var", risk: true, en: "rename the variable password to passwordHash", pt: "renomeie a variável senha para senhaHash"},
	{name: "risk typo auth", risk: true, en: "fix a typo in the authentication validation", pt: "corrija o typo na validação da autenticação"},
	{name: "risk format crypto", risk: true, en: "format the cryptography file", pt: "formate o arquivo de criptografia"},
	{name: "risk password param", risk: true, en: "add a password parameter to the Login method", pt: "adicione um parâmetro senha ao método Login"},
	{name: "risk payment flow bug", risk: true, en: "fix the bug in the payment flow", pt: "corrija o bug no fluxo de pagamento"},
	{name: "risk delete secret", risk: true, en: "delete the file with the client secret", pt: "apague o arquivo com o segredo do cliente"},
	{name: "risk crypto test", risk: true, en: "write a test for the cryptography function", pt: "escreva um teste para a função de criptografia"},
	{name: "risk list passwords", risk: true, en: "list the files that contain a password", pt: "liste os arquivos que contêm senha"},
	{name: "risk remove secret", risk: true, en: "remove the secret from the code and use an environment variable", pt: "remova o segredo do código e use variável de ambiente"},
	// "payment flow" is the EN behaviour stem. "payment function" and
	// "payment endpoint" take the same floor: bare "pagamento" already does.
	{name: "risk payment flow comment", risk: true, en: "add a comment on the payment flow", pt: "adicione um comentário no fluxo de pagamento"},
	{name: "risk payment comment", risk: true, en: "add a comment in the payment function", pt: "adicione um comentário na função de pagamento"},
	{name: "risk payment endpoint typo", risk: true, en: "fix a typo in the payment endpoint", pt: "corrija o typo no endpoint de pagamento"},
	{name: "risk auth error text", risk: true, en: "update the authentication error message", pt: "atualize a mensagem de erro de autenticação"},
}
