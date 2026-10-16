// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"slices"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestRiskPatterns_NoFalsePositives(t *testing.T) {
	for _, p := range []string{
		"rename the Race struct to Lap in the racing game",
		"fix typo in the race results leaderboard component",
		"rename races.go to events.go",
		"split this change into atomic commits",
		"rename the atomic CSS utility classes to tailwind ones",
		"move the Button into the atoms folder following atomic design",
		"renomeie o comando cobra clientes para customers",
		"a função cobre clientes inativos? adicione um teste",
		"corrija o typo no texto do certificado de participação",
		"adicione o campo dataEmissao ao certificado do curso",
		"renomeie a base de produtos para catalogo",
		"atualize o texto do banco de produtos na tela inicial",
		"truncate the title text in the table cell",
		"delete from the list the duplicated entries",
	} {
		if cats := core.RiskCategories(p); len(cats) > 0 {
			t.Errorf("%q matched %v", p, cats)
		}
	}
}

func TestRiskPatterns_StillRisky(t *testing.T) {
	for p, want := range map[string]string{
		"rename the variable and fix the race":                        "concurrency",
		"there is a data race in the cache":                           "concurrency",
		"use atomic.AddInt64 for the counter":                         "concurrency",
		"add a sync.WaitGroup to wait for workers":                    "concurrency",
		"replace the mutex with atomic operations":                    "concurrency",
		"make the map access thread-safe":                             "concurrency",
		"renomeie a variável no código que cobra o cartão do cliente": "payment",
		"cobrar o cartão do cliente duas vezes":                       "payment",
		"cobrança do cliente":                                         "payment",
		"desative a verificação do certificado":                       "crypto",
		"renovar o certificado digital":                               "crypto",
		"apague os dados de produção":                                 "destructive",
		"migração no banco de produção":                               "destructive",
		"truncate the orders table":                                   "destructive",
		"DELETE FROM users;":                                          "destructive",
		"excluir a tabela de usuários":                                "destructive",
		"dropar a tabela users":                                       "destructive",
		"rotacionar as chaves de API":                                 "secret",
		"rotacionar a chave privada":                                  "secret",
		"rotate the signing keys":                                     "secret",
		"rotate keys":                                                 "secret",
	} {
		if cats := core.RiskCategories(p); !slices.Contains(cats, want) {
			t.Errorf("RiskCategories(%q) = %v, want %s", p, cats, want)
		}
	}
}
