> **Histórico — 2026-10-22.** Este documento descreve um design anterior.
> A fonte viva é [`docs/design/architecture.md`](../design/architecture.md).

# Downshift — Arquitetura do Projeto

Visão técnica da estrutura, componentes e fluxo de dados do Downshift.

## Visão Geral

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              DOWNSHIFT                                       │
│                    Model Router for Coding Agents                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐    │
│  │   CLAUDE    │   │   CURSOR    │   │    CODEX    │   │  KIROCREW   │    │
│  │    CODE     │   │             │   │             │   │             │    │
│  └──────┬──────┘   └──────┬──────┘   └──────┬──────┘   └──────┬──────┘    │
│         │                 │                 │                 │            │
│         └─────────────────┴─────────────────┴─────────────────┘            │
│                                   │                                         │
│                                   ▼                                         │
│                    ┌──────────────────────────┐                            │
│                    │      HOOK LAYER          │                            │
│                    │   (preToolUse/postTool)  │                            │
│                    └────────────┬─────────────┘                            │
│                                 │                                           │
│         ┌───────────────────────┼───────────────────────┐                  │
│         │                       │                       │                  │
│         ▼                       ▼                       ▼                  │
│  ┌─────────────┐      ┌─────────────┐       ┌─────────────┐              │
│  │  CLASSIFY   │      │  COMPACT    │       │   ROUTE     │              │
│  │  (tier)     │      │  (output)   │       │  (model)    │              │
│  └─────────────┘      └─────────────┘       └─────────────┘              │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Estrutura de Diretórios

```
downshift/
├── cmd/                          # Comandos executáveis
│   ├── downshift/                # CLI principal
│   │   ├── main.go              # Entrypoint
│   │   ├── harnesses.go         # Registro de adapters
│   │   ├── setup_*.go           # Comandos setup por harness
│   │   └── *_test.go            # Testes de integração
│   ├── dsmon/                    # Monitor daemon (future)
│   └── dsmon-hook/               # Hook binary para harnesses
│
├── internal/                     # Packages privados
│   ├── core/                     # 🧠 Núcleo de decisão
│   │   ├── session.go           # Estado da sessão
│   │   ├── policy.go            # Políticas de roteamento
│   │   ├── escalation.go        # Lógica de escalação
│   │   ├── risk*.go             # Detecção de risco
│   │   └── *_test.go            # Corpus de testes
│   │
│   ├── classify/                 # 📊 Classificador de complexidade
│   │   ├── classifier.go        # Naive Bayes multinomial
│   │   ├── features.go          # Extração de features
│   │   └── tier.go              # Definição de tiers
│   │
│   ├── compressor/              # 📦 Compactação de output
│   │   ├── compressor.go        # Engine principal
│   │   ├── rules/               # Regras por tipo de comando
│   │   └── learning/            # Aprendizado de padrões
│   │
│   ├── adapters/                # 🔌 Integrações com harnesses
│   │   ├── claudecode/          # Claude Code
│   │   ├── cursor/              # Cursor
│   │   ├── codex/               # OpenAI Codex
│   │   ├── kirocrew/            # Kiro Crew
│   │   ├── opencode/            # OpenCode
│   │   └── antigravity/         # Antigravity
│   │
│   ├── gateway/                 # 🌐 Gateway HTTP (future)
│   │   ├── omniroute/           # Router unificado
│   │   └── openrouter/          # OpenRouter backend
│   │
│   ├── catalog/                 # 📚 Catálogo de modelos
│   │   ├── models.go            # Definições de modelo
│   │   └── tiers.go             # Mapping tier→modelo
│   │
│   ├── quota/                   # 💰 Gestão de quota
│   │   ├── quota.go             # Tracking de uso
│   │   └── limits.go            # Limites por tier
│   │
│   ├── session/                 # 📝 Estado persistente
│   │   ├── store.go             # Armazenamento
│   │   └── metrics.go           # Métricas de sessão
│   │
│   ├── semantic/                # 🔤 Análise semântica
│   │   └── fuse.go              # Fusível semântico
│   │
│   ├── discovery/               # 🔍 Auto-descoberta
│   │   └── harness.go           # Detecta harness instalado
│   │
│   ├── telemetry/               # 📈 Observabilidade
│   │   ├── events.go            # Eventos estruturados
│   │   └── export.go            # Exportação de dados
│   │
│   └── models/                  # 📋 Modelos de dados
│       └── types.go             # Tipos compartilhados
│
├── docs/                        # 📖 Documentação
│   ├── architecture/            # Arquitetura (este doc)
│   ├── reference/               # Referência técnica
│   ├── guides/                  # Guias de uso
│   └── evidence/                # Evidências e métricas
│
├── evals/                       # 🧪 Avaliações
│   └── corpus/                  # Corpus de teste
│
└── scripts/                     # 🛠️ Scripts auxiliares
    └── ci-*.sh                  # Scripts de CI
```

## Fluxo de Dados

### 1. Hook preToolUse (Classificação)

```
┌─────────────────────────────────────────────────────────────────┐
│                    preToolUse HOOK FLOW                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Harness (Claude Code, etc)                                    │
│       │                                                         │
│       │ tool_use request                                        │
│       ▼                                                         │
│  ┌─────────────┐                                               │
│  │ dsmon-hook  │ ◀── Binary embarcado no harness               │
│  └──────┬──────┘                                               │
│         │                                                       │
│         ▼                                                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    CLASSIFY PIPELINE                     │   │
│  ├─────────────────────────────────────────────────────────┤   │
│  │                                                         │   │
│  │  1. Extract features ──▶ TokenBag                       │   │
│  │         │                                               │   │
│  │         ▼                                               │   │
│  │  2. Naive Bayes ──▶ P(tier|features)                   │   │
│  │         │                                               │   │
│  │         ▼                                               │   │
│  │  3. Risk Check ──▶ floor_tier                          │   │
│  │         │                                               │   │
│  │         ▼                                               │   │
│  │  4. Policy Apply ──▶ final_tier, model_id              │   │
│  │                                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│         │                                                       │
│         ▼                                                       │
│  Response: {allow: true, model_override?: "..."}               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2. Compaction Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                    COMPACTION PIPELINE                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  $ downshift compactor -- go test ./...                        │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────┐                                               │
│  │ execwrap    │ ◀── Executa comando, captura output           │
│  └──────┬──────┘                                               │
│         │                                                       │
│         ▼                                                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                 COMPRESSOR ENGINE                        │   │
│  ├─────────────────────────────────────────────────────────┤   │
│  │                                                         │   │
│  │  1. Detect command family ──▶ "go test"                 │   │
│  │         │                                               │   │
│  │         ▼                                               │   │
│  │  2. Select rules ──▶ GoTestRules                        │   │
│  │         │                                               │   │
│  │         ▼                                               │   │
│  │  3. Parse output ──▶ Structured result                  │   │
│  │         │                                               │   │
│  │         ▼                                               │   │
│  │  4. Extract essentials:                                 │   │
│  │     • PASS/FAIL status                                  │   │
│  │     • Error messages (if FAIL)                          │   │
│  │     • Summary line                                      │   │
│  │     • Coverage (if present)                             │   │
│  │         │                                               │   │
│  │         ▼                                               │   │
│  │  5. Format compact output                               │   │
│  │                                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│         │                                                       │
│         ▼                                                       │
│  Output: "✓ PASS  50 packages  0.8s  (82% smaller)"            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 3. Session State Machine

```
┌─────────────────────────────────────────────────────────────────┐
│                    SESSION STATE MACHINE                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────┐   start    ┌─────────┐   classify   ┌─────────┐  │
│  │  IDLE   │ ─────────▶ │ ACTIVE  │ ───────────▶ │ ROUTED  │  │
│  └─────────┘            └─────────┘              └─────────┘  │
│       ▲                      │                        │        │
│       │                      │ timeout                │        │
│       │                      ▼                        │        │
│       │                ┌─────────┐                    │        │
│       └────────────────│ STALE   │◀───────────────────┘        │
│          cleanup       └─────────┘     complete                │
│                                                                 │
│  Persisted State:                                              │
│  • session_id                                                  │
│  • harness_type                                                │
│  • decisions[]                                                 │
│  • quota_used                                                  │
│  • patterns_learned                                            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Componentes Principais

### Core (`internal/core`)

O núcleo de decisão do Downshift:

```go
// session.go - Estado da sessão
type Session struct {
    ID          string
    Harness     HarnessType
    StartedAt   time.Time
    Decisions   []Decision
    QuotaUsed   TokenCount
    CurrentTier Tier
}

// policy.go - Políticas de roteamento
type Policy struct {
    DefaultTier     Tier
    RiskFloor       Tier
    CostCeiling     float64
    AllowDownshift  bool
}

// escalation.go - Escalação automática
func (s *Session) ShouldEscalate(ctx ClassifyContext) bool {
    return ctx.Risk.Critical || ctx.Confidence < s.policy.ConfidenceThreshold
}
```

### Classify (`internal/classify`)

Classificador Naive Bayes determinístico:

```go
// classifier.go
type Classifier struct {
    model    *NaiveBayesModel
    features *FeatureExtractor
}

func (c *Classifier) Classify(prompt string) (Tier, float64, []RiskFlag) {
    features := c.features.Extract(prompt)
    probs := c.model.Predict(features)
    tier := c.selectTier(probs)
    risk := c.detectRisk(features)
    return tier, probs[tier], risk
}
```

### Compressor (`internal/compressor`)

Engine de compactação:

```go
// compressor.go
type Compressor struct {
    rules    RuleSet
    learning *LearningStore
}

func (c *Compressor) Compact(cmd string, output []byte) ([]byte, Stats) {
    family := c.detectFamily(cmd)
    rule := c.rules.For(family)
    if rule == nil {
        return output, Stats{Passthrough: true}
    }
    return rule.Apply(output)
}
```

### Adapters (`internal/adapters`)

Cada adapter implementa a interface do harness:

```go
// adapter.go
type Adapter interface {
    Name() string
    Setup(cfg Config) error
    PreToolUse(ctx HookContext) (*HookResponse, error)
    PostToolUse(ctx HookContext) error
    Capabilities() Capabilities
}

// Implementações:
// - claudecode.Adapter
// - cursor.Adapter
// - codex.Adapter
// - kirocrew.Adapter
// - opencode.Adapter
// - antigravity.Adapter
```

## Integrações

### Harness Hooks

```
┌────────────────────────────────────────────────────────────────────────────┐
│                          HARNESS INTEGRATIONS                              │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐           │
│  │   CLAUDE CODE   │  │     CURSOR      │  │    KIROCREW     │           │
│  │   ~/.claude/    │  │  ~/.cursor/     │  │  ~/.kiro/       │           │
│  │   hooks.json    │  │  hooks.json     │  │  config.json    │           │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘           │
│           │                    │                    │                     │
│           └────────────────────┼────────────────────┘                     │
│                                │                                          │
│                                ▼                                          │
│                    ┌─────────────────────┐                               │
│                    │    dsmon-hook       │                               │
│                    │  (binary comum)     │                               │
│                    └─────────────────────┘                               │
│                                │                                          │
│                                ▼                                          │
│                    ┌─────────────────────┐                               │
│                    │    DOWNSHIFT CLI    │                               │
│                    │  - classify         │                               │
│                    │  - compact          │                               │
│                    │  - route            │                               │
│                    └─────────────────────┘                               │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

### Gateway HTTP (Future)

```
┌────────────────────────────────────────────────────────────────────────────┐
│                          GATEWAY MODE (FUTURE)                             │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  Client ──▶ POST /v1/chat/completions                                     │
│                    │                                                       │
│                    ▼                                                       │
│           ┌─────────────────┐                                             │
│           │  DOWNSHIFT GW   │                                             │
│           │  (HTTP proxy)   │                                             │
│           └────────┬────────┘                                             │
│                    │                                                       │
│         ┌──────────┼──────────┐                                           │
│         │          │          │                                           │
│         ▼          ▼          ▼                                           │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐                               │
│  │ Anthropic │ │ OpenRouter│ │  OpenAI   │                               │
│  │    API    │ │    API    │ │   API     │                               │
│  └───────────┘ └───────────┘ └───────────┘                               │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

## Métricas e Observabilidade

```
┌────────────────────────────────────────────────────────────────────────────┐
│                          TELEMETRY                                         │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  Métricas coletadas:                                                      │
│  ├── classify_latency_ms        Tempo de classificação                    │
│  ├── compact_ratio              Taxa de compressão                        │
│  ├── tier_distribution          Distribuição de tiers                     │
│  ├── downshift_count            Decisões de downshift                     │
│  ├── escalation_count           Escalações por risco                      │
│  ├── quota_usage                Uso de quota                              │
│  └── session_duration           Duração de sessões                        │
│                                                                            │
│  Comandos:                                                                 │
│  $ downshift stats              Estatísticas globais                      │
│  $ downshift session            Sessão atual                              │
│  $ downshift report --period=7d Relatório de período                      │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

## Princípios de Design

1. **No LLM in the loop**: Classificação é determinística (Naive Bayes)
2. **Safety-first**: Risco sempre eleva tier, nunca reduz
3. **Harness-agnostic**: Core funciona com qualquer coding agent
4. **Observable**: Todas as decisões são logadas e explicáveis
5. **Incremental**: Compactação e routing são independentes

## Referências

- BDI Architecture: [docs/architecture/agentic-bdi.md](agentic-bdi.md)
- Beta Exit Criteria: [docs/reference/beta-exit.md](../reference/beta-exit.md)
- Harness Matrix: [docs/reference/harness-matrix.md](../reference/harness-matrix.md)

---

Downshift by Tiago Vilas Boas  
https://github.com/tiagovilasboas/downshift
