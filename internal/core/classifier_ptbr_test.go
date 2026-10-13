// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// PT-BR mirror of the TRIVIAL/SIMPLE signals. Cheap Portuguese prompts must
// leave the medium default, hard ones must still reach frontier, and risk
// prompts must keep their floor.
//
// harness-downshift by Tiago de Carvalho Vilas Boas
// https://github.com/tiagovilasboas/downshift

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

var ptbrCheap = []string{
	"Atualize a descrição do botão de login",
	"Escreva um resumo curto do PR",
	"Renomeie a variável userId para userIdentifier",
	"Corrija o typo no README",
	"Formate o arquivo",
	"Adicione um comentário na função",
	"Liste os arquivos do diretório",
	"Formate o arquivo main.go",
	"Indente o bloco de configuração",
	"Renomeie a função calcTotal para computeTotal",
	"Corrija um typo no changelog",
	"Corrija o erro de digitação no título da página",
	"Adicione um comentário explicando o loop",
	"Apague o arquivo temp.txt",
	"Mova o arquivo utils.go para a pasta helpers",
	"Remova o código morto do módulo de datas",
	"Adicione um campo nome na struct Usuario",
	"Adicione um parâmetro timeout ao método Fetch",
	"Corrija o bug do contador que não zera",
	"Escreva uma função que soma dois números",
	"Escreva um teste para a função Soma",
	"Atualize o texto do botão de cancelar",
	"Ajuste a mensagem de erro do formulário",
	"Altere o rótulo do campo de busca",
	"Escreva um resumo breve das mudanças",
	"Liste os arquivos modificados no último commit",
	"Mostre os arquivos de log do diretório tmp",
	"Explique o que esta regex faz",
	"Formate o código com prettier",
	"Rode os testes e mostre o resultado",
}

var ptbrTrivialOrSimple = []string{
	"Renomeie a variável userId para userIdentifier",
	"Corrija o typo no README",
	"Formate o arquivo",
	"Adicione um comentário na função",
	"Liste os arquivos do diretório",
	"Apague o arquivo temp.txt",
	"Adicione um campo nome na struct Usuario",
	"Escreva um resumo curto do PR",
	"Atualize a descrição do botão de login",
	"Corrija o bug do contador que não zera",
}

var ptbrHard = []string{
	"Redesenhe a arquitetura do serviço de notificações",
	"Planeje a migração do banco de dados para outra região sem downtime",
	"Faça uma auditoria de segurança completa do fluxo de upload",
	"Investigue a condição de corrida no worker de filas",
	"Projete um sistema distribuído de cache com replicação",
	"Rearquitete toda a plataforma de relatórios",
	"Migre toda a base de código de JavaScript para TypeScript",
	"Redesenhe a arquitetura e formate o arquivo de configuração",
	"Migre o serviço de pedidos para microsserviços sem downtime e escreva um teste",
	"Revise a arquitetura do sistema distribuído de mensageria",
	"Arquitete uma plataforma multi-tenant com consenso entre nós",
}

var ptbrRisk = []string{
	"Renomeie a variável password para senhaHash",
	"Corrija o typo na validação da autenticação",
	"Formate o arquivo de criptografia",
	"Adicione um comentário na função de pagamento",
	"Adicione um parâmetro senha ao método Login",
	"Corrija o bug no fluxo de pagamento",
	"Atualize a mensagem de erro de autenticação",
	"Apague o arquivo com o segredo do cliente",
	"Escreva um teste para a função de criptografia",
	"Liste os arquivos que contêm senha",
	"Remova o segredo do código e use variável de ambiente",
	"Corrija o typo no endpoint de pagamento",
}

func TestPTBRCheapStaysCheap(t *testing.T) {
	corpusEnv(t, "")
	for _, p := range ptbrCheap {
		t.Run(p, func(t *testing.T) {
			if d := corpusRoute(p); d.Tier > core.TierMid {
				t.Errorf("tier=%s complexity=%s", d.Tier, d.Complexity)
			}
		})
	}
}

func TestPTBRCheapLeavesMedium(t *testing.T) {
	corpusEnv(t, "")
	for _, p := range ptbrTrivialOrSimple {
		t.Run(p, func(t *testing.T) {
			c := core.Classify(p)
			if c.Complexity != core.Trivial && c.Complexity != core.Simple {
				t.Errorf("complexity=%s scores=%v", c.Complexity, c.Scores)
			}
		})
	}
}

func TestPTBRHardStaysFrontier(t *testing.T) {
	corpusEnv(t, "")
	for _, p := range ptbrHard {
		t.Run(p, func(t *testing.T) {
			if d := corpusRoute(p); d.Tier != core.TierFrontier {
				t.Errorf("tier=%s complexity=%s", d.Tier, d.Complexity)
			}
		})
	}
}

func TestPTBRRiskKeepsFloor(t *testing.T) {
	corpusEnv(t, "")
	for _, p := range ptbrRisk {
		t.Run(p, func(t *testing.T) {
			if d := corpusRoute(p); d.Tier < core.TierMid {
				t.Errorf("risk prompt below mid: tier=%s risk=%v", d.Tier, d.Risk)
			}
		})
	}
}
