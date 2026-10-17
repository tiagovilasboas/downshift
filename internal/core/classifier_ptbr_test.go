// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

// PT-BR mirror of the TRIVIAL/SIMPLE signals. Cheap Portuguese prompts must
// leave the medium default, hard ones must still reach frontier, and risk
// prompts must keep their floor.
//
// Downshift by Tiago de Carvalho Vilas Boas
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

// TestPTBRVocabularyParity is the English/Portuguese signal mirror. Each
// prompt is the PT-BR twin of a RawSignals stem and must land on the same
// complexity. Phrases that also hit a risk word or a higher class are not
// listed here.
func TestPTBRVocabularyParity(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   core.Complexity
	}{
		{name: "reescreva", prompt: "reescreva o parser", want: core.Complex},
		{name: "revisão de código", prompt: "faça uma revisão de código sem mudar nada", want: core.Complex},
		{name: "documento de design", prompt: "escreva um documento de design para o novo fluxo de checkout", want: core.Complex},
		{name: "distribuído", prompt: "corrija o bug no processador distribuído", want: core.Complex},
		{name: "cache distribuído", prompt: "projete um cache distribuído para o agendador", want: core.Complex},
		{name: "por que falha", prompt: "por que o worker falha sob carga", want: core.Complex},
		{name: "pense a fundo", prompt: "pense a fundo nos modos de falha", want: core.Complex},
		{name: "vazamento de memória", prompt: "depure o vazamento de memória no pool de conexões sob carga alta", want: core.Complex},
		{name: "impasse", prompt: "há um impasse no worker de filas", want: core.Complex},
		{name: "bloqueio mútuo", prompt: "há um bloqueio mútuo no worker de filas", want: core.Complex},
		{name: "multi-inquilino", prompt: "adicione isolamento multi-inquilino ao banco de relatórios", want: core.Complex},
		{name: "multi-região", prompt: "planeje o failover multi-região do serviço de notificações", want: core.Complex},
		{name: "entre sistemas", prompt: "corrija a falha entre sistemas de pedido e estoque", want: core.Complex},
		{name: "compensações", prompt: "descreva as compensações entre o plugin no processo e o plugin externo", want: core.Complex},
		{name: "projete um sistema", prompt: "projete um sistema de filas para o agendador", want: core.Complex},
		{name: "construa uma plataforma", prompt: "construa uma plataforma de colaboração para o time", want: core.Complex},
		{name: "grafo", prompt: "modele o grafo de dependências entre pacotes", want: core.Complex},
		{name: "heap binário", prompt: "use um heap binário na fila de prioridade", want: core.Complex},
		{name: "concorrente", prompt: "processe os lotes de forma concorrente sem reordenar eventos", want: core.Complex},
		{name: "consenso", prompt: "o cluster precisa de consenso entre os nós", want: core.Complex},
		{name: "replicação", prompt: "o cache precisa de replicação entre os nós", want: core.Complex},
		{name: "falha em cascata", prompt: "analise a falha em cascata do incidente de ontem", want: core.Complex},
		{name: "malha de serviços", prompt: "implante uma malha de serviços com descoberta", want: core.Complex},
		{name: "engenharia do caos", prompt: "rode engenharia do caos no cluster de teste", want: core.Complex},
		{name: "linhagem de dados", prompt: "registre a linhagem de dados do pipeline de vendas", want: core.Complex},
		{name: "entre clusters", prompt: "sincronize o estado entre clusters sem perder mensagens", want: core.Complex},
		{name: "pipeline de streaming", prompt: "ajuste o pipeline de streaming de eventos de clique", want: core.Complex},
		{name: "serving de modelos", prompt: "configure o serving de modelos no cluster", want: core.Complex},
		{name: "colaboração em tempo real", prompt: "implemente a edição com colaboração em tempo real", want: core.Complex},
		{name: "descida recursiva", prompt: "escreva um parser de descida recursiva", want: core.Complex},
		{name: "subsequência comum", prompt: "calcule a subsequência comum mais longa das duas strings", want: core.Complex},
		{name: "precedência de operadores", prompt: "respeite a precedência de operadores na expressão", want: core.Complex},
		{name: "operadores postfixos", prompt: "implemente operadores postfixos no parser", want: core.Complex},
		{name: "refatore", prompt: "refatore o logger e adicione um campo", want: core.Medium},
		{name: "implemente", prompt: "implemente o retry e corrija o bug", want: core.Medium},
		{name: "integre", prompt: "integre o cliente novo e corrija o bug", want: core.Medium},
		{name: "funcionalidade", prompt: "adicione a funcionalidade de exportar CSV e corrija o bug", want: core.Medium},
		{name: "em vários arquivos", prompt: "atualize o estilo em quatro arquivos e adicione um campo", want: core.Medium},
		{name: "depure", prompt: "depure o problema e corrija o bug", want: core.Medium},
		{name: "schema online", prompt: "aplique a alteração de schema online e corrija o bug", want: core.Medium},
		{name: "única função", prompt: "altere uma única função para retornar erro", want: core.Simple},
		{name: "qual é", prompt: "qual é o propósito do flag NoSignal", want: core.Simple},
		{name: "quais são", prompt: "quais são as diferenças entre slice e array", want: core.Simple},
		{name: "o que faz", prompt: "o que a função Plan faz", want: core.Simple},
		{name: "página de destino", prompt: "crie uma página de destino com um botão", want: core.Simple},
		{name: "olá mundo", prompt: "crie um olá mundo em Rust", want: core.Simple},
		{name: "cubo girando", prompt: "crie um cubo girando com three.js", want: core.Simple},
		{name: "faça o commit", prompt: "faça o commit das mudanças e o push", want: core.Trivial},
		{name: "código repetitivo", prompt: "gere o código repetitivo de um subcomando", want: core.Trivial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := core.Classify(tt.prompt)
			if got.Complexity != tt.want {
				t.Errorf("complexity=%s scores=%v risk=%v, want %s", got.Complexity, got.Scores, got.Risk, tt.want)
			}
		})
	}
}

// TestPTBRVocabularyBoundaries keeps look-alike Portuguese wording off the
// class the new alternations vote for.
func TestPTBRVocabularyBoundaries(t *testing.T) {
	belowComplex := []string{
		"desenhe o formulário de login",
		"atualize o projeto do sistema de build",
		"construa o site de documentação",
		"construa um sistema de filas simples",
		"replicar o bug localmente e adicione um teste",
		"adicione um gráfico de uso semanal ao README",
		"distribuir os arquivos entre as pastas",
	}
	for _, p := range belowComplex {
		t.Run(p, func(t *testing.T) {
			if got := core.Classify(p); got.Complexity == core.Complex {
				t.Errorf("complexity=%s scores=%v, want below COMPLEX", got.Complexity, got.Scores)
			}
		})
	}
	stayMedium := []string{
		"por que o checkout está lento",
		"descubra por que o CLI imprime uma tabela vazia",
	}
	for _, p := range stayMedium {
		t.Run(p, func(t *testing.T) {
			if got := core.Classify(p); got.Complexity != core.Medium {
				t.Errorf("complexity=%s scores=%v, want MEDIUM", got.Complexity, got.Scores)
			}
		})
	}
}

// TestShortNudgeDoesNotOutvoteComplex pins the six-word nudge: a Portuguese
// sentence one word shorter than its English twin must not turn a Complex tie
// into Trivial.
func TestShortNudgeDoesNotOutvoteComplex(t *testing.T) {
	en := core.Classify("format the output of the rearchitected pipeline")
	pt := core.Classify("formate a saída do pipeline rearquitetado")
	if en.Complexity != core.Complex || pt.Complexity != core.Complex {
		t.Fatalf("en=%s scores=%v pt=%s scores=%v, want both COMPLEX", en.Complexity, en.Scores, pt.Complexity, pt.Scores)
	}
}
