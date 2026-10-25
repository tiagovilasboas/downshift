# Downshift v0.2.0 Roadmap — KiroCrew Deep Integration

**Status:** ✅ Released (2026-10-25)
**Theme:** Leverage F8 discoveries for performance metrics

## Background

O REA profundo (F8) revelou que o KiroCrew é Electron + Python backend completo.
Isso abre oportunidades de integração que não dependem de mudanças no produto.

## Features Entregues

### 1. ✅ Savings Estimation via context_used (2 pts)

**O que:** Estimar economia de tokens baseado em output compactado vs original.

**Implementação:**
```go
// internal/discovery/kirocrew.go
type KiroSessionMetrics struct {
    ContextUsed   int64  `json:"context_used"`
    ContextWindow int64  `json:"context_window"`
    DurationMs    int64  `json:"duration_ms"`
    Model         string `json:"model"`
}

func InferKiroCrewMetrics(sessionID string) (KiroSessionMetrics, error)
func (m KiroSessionMetrics) ContextUtilization() float64

// internal/accounting/kirocrew.go
func EstimateContextSavings(compactorSaved int64, contextUsed int64) float64
```

**Dados utilizados:**
- `context_used` em `~/.kiro/sessions/cli/<id>.json`
- `context_window` para calcular utilização

### 2. ✅ Harness Limitations Registry (novo)

**O que:** Registry centralizado de limitações por harness para transparência.

**Implementação:**
```go
// internal/limits/limits.go
const (
    FeatureModelRewrite      = "model_rewrite"
    FeatureTokenCounts       = "token_counts"
    FeatureCompaction        = "compaction"
    FeatureMidSessionRouting = "mid_session_routing"
    FeatureSavingsEstimation = "savings_estimation"
)

const (
    StatusFull    = "full"
    StatusPartial = "partial"
    StatusNone    = "none"
    StatusManual  = "manual"
)

func ForHarness(harnessID string) *HarnessLimits
```

**Harnesses cobertos:** kirocrew, claudecode, cursor, codex, antigravity, opencode

### 3. ✅ Doctor Integration

**O que:** Mostrar limitações no output do doctor.

**Exemplo:**
```
kirocrew
  ✓ hooks wired     2 of 2 expected
  ✓ session list    file (3 models)
  ! limitations     model_rewrite:none, token_counts:none, compaction:manual
      fix: see 'downshift help kirocrew' for details
```

## Features Movidas para v0.3.0

### Model-Aware Routing Advisory (3 pts)
- Recomendar modelo baseado no tipo de comando e contexto
- Flag `--recommend-model` no compactor

### Metrics Dashboard Aprimorado (3 pts)
- Visualização de métricas acumuladas
- `downshift dashboard` ou `downshift stats --week`

### preToolUse Hook Integration (5 pts)
- Hook nativo no KiroCrew para métricas em tempo real
- `downshift hook --install` para setup automático

## Timeline

```
v0.1.4 (2026-10-25) ────────────────────────────────────────
  ✅ KiroCrew harness detection
  ✅ Model inference
  ✅ Compactor steering

v0.2.0 (2026-10-25) ────────────────────────────────────────
  ✅ Savings estimation (context_used proxy)
  ✅ Harness limitations registry
  ✅ Doctor integration

v0.3.0 (próximo) ───────────────────────────────────────────
  🔲 Model advisory
  🔲 Metrics dashboard aprimorado
  🔲 preToolUse hook nativo
```

## Bloqueios Conhecidos (KiroCrew)

| Feature | Bloqueio | Workaround |
|---------|----------|------------|
| Token counting real | ACP não expõe tokens | Usar context_used como proxy |
| Cost calculation real | Sem tokens, sem custo | Estimativa baseada em context |
| Auto model switching | preToolUse é allow/block only | Policy mode (block + retry) |
| Automatic compaction | Sem updated_input | Steering para invocação manual |

## PRs Incluídos

- #565: docs: add v0.2.0 roadmap
- #566: feat(accounting): add context metrics for savings estimation

## Documentação Relacionada

- F1-F8 findings: `downshift-labs/reverse/kirocrew/findings/`
- SPIKE: `downshift-labs/reverse/kirocrew/SPIKE.md`
- harness-registry: seção "REA: KiroCrew Integration"
