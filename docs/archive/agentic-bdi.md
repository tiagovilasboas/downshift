> **Histórico — 2026-10-22.** Este documento descreve um design anterior.
> A fonte viva é [`docs/design/architecture.md`](../design/architecture.md).

# Downshift — Arquitetura Agentic AI BDI

Este documento descreve como o Downshift implementa uma arquitetura inspirada em
**BDI (Belief-Desire-Intention)** para roteamento inteligente de modelos, sem
usar um LLM no loop de decisão.

## Modelo BDI Aplicado ao Roteamento

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           DOWNSHIFT BDI AGENT                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐         │
│  │    BELIEFS      │    │    DESIRES      │    │   INTENTIONS    │         │
│  │   (Percepção)   │───▶│    (Metas)      │───▶│    (Ações)      │         │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘         │
│         │                      │                      │                     │
│         ▼                      ▼                      ▼                     │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐         │
│  │ • Task context  │    │ • Cost minimize │    │ • Route to tier │         │
│  │ • Risk signals  │    │ • Quality keep  │    │ • Compact output│         │
│  │ • Session state │    │ • Safety ensure │    │ • Learn pattern │         │
│  │ • Harness caps  │    │ • Speed maximize│    │ • Escalate risk │         │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘         │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Beliefs (Crenças) — Percepção do Mundo

O agente forma crenças sobre a tarefa atual através de múltiplos sensores:

### 1. Classificador de Complexidade (`internal/classify`)

```
Input: prompt do usuário + contexto
       ↓
┌──────────────────────────────────────┐
│  Naive Bayes Multinomial Classifier  │
│  (determinístico, sem LLM)           │
├──────────────────────────────────────┤
│  Features extraídas:                 │
│  • Tokens de domínio técnico         │
│  • Padrões de código/linguagem       │
│  • Indicadores de complexidade       │
│  • Sinais de risco (pagamento, auth) │
└──────────────────────────────────────┘
       ↓
Output: Tier (T0-T4) + confidence + risk_flags
```

**Tiers de Complexidade:**

| Tier | Nome | Descrição | Modelo típico |
|------|------|-----------|---------------|
| T0 | Trivial | Formatação, lookup simples | Haiku |
| T1 | Simple | Bug fix 1-3 arquivos, feature definida | Sonnet |
| T2 | Medium | Multi-arquivo, integração | Sonnet+ |
| T3 | Complex | Arquitetura, multi-camada | Opus |
| T4 | Critical | Decisões irreversíveis, segurança | Opus (pinned) |

### 2. Detector de Risco (`internal/core/risk`)

```
┌─────────────────────────────────────────┐
│         RISK SIGNAL DETECTION           │
├─────────────────────────────────────────┤
│                                         │
│  [Payment]──────┐                       │
│  [Auth/Secrets]─┼──▶ Risk Aggregator    │
│  [Destructive]──┤         │             │
│  [Production]───┘         ▼             │
│                    ┌─────────────┐      │
│                    │ TIER FLOOR  │      │
│                    │ ENFORCEMENT │      │
│                    └─────────────┘      │
│                                         │
│  Se risco detectado: tier ≥ T3         │
│  Nunca downshift em contexto crítico   │
│                                         │
└─────────────────────────────────────────┘
```

### 3. Estado da Sessão (`internal/session`)

- Histórico de decisões da sessão
- Quota consumida vs disponível
- Padrões de uso recente
- Contexto do harness (Claude Code, Cursor, etc.)

## Desires (Desejos) — Objetivos do Agente

O agente opera com múltiplos desejos que podem entrar em conflito:

```
┌─────────────────────────────────────────────────────────────────┐
│                    DESIRE HIERARCHY                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  PRIORITY 1: Safety (inviolável)                                │
│  ├── Nunca rotear tarefa crítica para modelo barato             │
│  ├── Preservar qualidade em código de produção                  │
│  └── Escalar automaticamente em sinais de risco                 │
│                                                                  │
│  PRIORITY 2: Quality (threshold mínimo)                         │
│  ├── Manter acceptance rate acima do baseline                   │
│  ├── Não degradar casos que funcionavam                         │
│  └── Preservar capacidade comprovada                            │
│                                                                  │
│  PRIORITY 3: Efficiency (otimização)                            │
│  ├── Minimizar custo por tarefa aceita                          │
│  ├── Reduzir tokens consumidos via compactação                  │
│  └── Acelerar tarefas triviais com modelos rápidos              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Resolução de Conflitos

```
if risk_detected:
    desire = SAFETY  # sempre vence
elif confidence < threshold:
    desire = QUALITY  # não arriscar
else:
    desire = EFFICIENCY  # otimizar custo
```

## Intentions (Intenções) — Plano de Ação

Com base nas crenças e desejos, o agente forma intenções concretas:

### 1. Decisão de Roteamento

```
┌──────────────────────────────────────────────────────────────────┐
│                     ROUTING DECISION TREE                        │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  classify(prompt) ──▶ tier, confidence, risk                    │
│         │                                                        │
│         ▼                                                        │
│  ┌─────────────────┐                                            │
│  │ risk.critical?  │──YES──▶ INTENTION: pin_opus                │
│  └────────┬────────┘                                            │
│           │ NO                                                   │
│           ▼                                                      │
│  ┌─────────────────┐                                            │
│  │ confidence < θ? │──YES──▶ INTENTION: use_current_model       │
│  └────────┬────────┘         (não downshift com incerteza)      │
│           │ NO                                                   │
│           ▼                                                      │
│  ┌─────────────────┐                                            │
│  │ tier < current? │──YES──▶ INTENTION: downshift_to_tier       │
│  └────────┬────────┘                                            │
│           │ NO                                                   │
│           ▼                                                      │
│  INTENTION: maintain_current                                     │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 2. Compactação de Output

```
┌──────────────────────────────────────────────────────────────────┐
│                    OUTPUT COMPACTION                             │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Comando executado ──▶ Output bruto (pode ser enorme)           │
│         │                                                        │
│         ▼                                                        │
│  ┌─────────────────┐                                            │
│  │ É compactável?  │                                            │
│  │ (test runner,   │──YES──▶ INTENTION: compact_output          │
│  │  gh cli, etc)   │              │                             │
│  └────────┬────────┘              ▼                             │
│           │ NO              ┌───────────────┐                   │
│           │                 │ Extrair:      │                   │
│           │                 │ • PASS/FAIL   │                   │
│           │                 │ • Summary     │                   │
│           │                 │ • Errors only │                   │
│           │                 └───────────────┘                   │
│           ▼                        │                             │
│  INTENTION: pass_through           │                             │
│  (output sem modificação)          ▼                             │
│                             Output compactado                    │
│                             (82-97% menor)                       │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 3. Learning Loop (Aprendizado)

```
┌──────────────────────────────────────────────────────────────────┐
│                    LEARNING FEEDBACK LOOP                        │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Decisão tomada ──▶ Outcome observado                           │
│         │                  │                                     │
│         │                  ▼                                     │
│         │          ┌─────────────────┐                          │
│         │          │ Foi aceito?     │                          │
│         │          │ Houve retry?    │                          │
│         │          │ Humano corrigiu?│                          │
│         │          └────────┬────────┘                          │
│         │                   │                                    │
│         │                   ▼                                    │
│         │          ┌─────────────────┐                          │
│         └─────────▶│ Pattern Store   │                          │
│                    │ (file-based)    │                          │
│                    └────────┬────────┘                          │
│                             │                                    │
│                             ▼                                    │
│                    INTENTION: adjust_future_routing              │
│                    (padrões aprendidos informam beliefs)        │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

## Ciclo BDI Completo

```
                    ┌──────────────────────────────┐
                    │      ENVIRONMENT             │
                    │  (Coding Agent Harness)      │
                    └──────────────┬───────────────┘
                                   │
                                   ▼
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐     ┌─────────┐  │
│   │ PERCEPT │────▶│ BELIEFS │────▶│ DESIRES │────▶│INTENTION│  │
│   └─────────┘     └─────────┘     └─────────┘     └─────────┘  │
│        │                                               │         │
│        │                                               ▼         │
│        │                                        ┌───────────┐   │
│        │                                        │  ACTION   │   │
│        │                                        │ EXECUTION │   │
│        │                                        └─────┬─────┘   │
│        │                                              │         │
│        │              ┌───────────────────────────────┘         │
│        │              │                                          │
│        │              ▼                                          │
│        │       ┌─────────────┐                                  │
│        └───────│   OUTCOME   │                                  │
│                │  FEEDBACK   │                                  │
│                └─────────────┘                                  │
│                                                                  │
│                      DOWNSHIFT BDI AGENT                        │
└──────────────────────────────────────────────────────────────────┘
```

## Propriedades Emergentes

O design BDI do Downshift resulta em comportamentos emergentes:

1. **Autonomia**: decide tier sem consultar LLM
2. **Reatividade**: responde imediatamente a sinais de risco
3. **Proatividade**: otimiza custo quando seguro
4. **Adaptabilidade**: aprende padrões do projeto específico
5. **Transparência**: decisões explicáveis e auditáveis

## Diferenças do BDI Clássico

| Aspecto | BDI Clássico | Downshift BDI |
|---------|--------------|---------------|
| Raciocínio | Lógico/simbólico | Estatístico (Naive Bayes) |
| Ambiente | Simulado/fechado | Real (coding agents) |
| Goals | Explícitos | Implícitos (cost/quality/safety) |
| Execução | Própria | Delegada ao LLM target |
| Feedback | Simulado | Real (acceptance, retries) |

## Referências

- Rao, A. S., & Georgeff, M. P. (1995). BDI Agents: From Theory to Practice.
- Bratman, M. E. (1987). Intention, Plans, and Practical Reason.
- Downshift Corpus Tests: `internal/core/classifier_corpus_test.go`
- Risk Safety Tests: `internal/core/risk_safety_test.go`

---

Downshift by Tiago Vilas Boas  
https://github.com/tiagovilasboas/downshift
