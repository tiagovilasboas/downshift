# Downshift v0.2.0 Roadmap — KiroCrew Deep Integration

**Status:** Planning
**Target:** After v0.1.4 stabilizes
**Theme:** Leverage F8 discoveries for performance metrics

## Background

O REA profundo (F8) revelou que o KiroCrew é Electron + Python backend completo.
Isso abre oportunidades de integração que não dependem de mudanças no produto.

## Features Planejadas

### 1. Savings Estimation via context_used (2 pts)

**O que:** Estimar economia de tokens baseado em output compactado vs original.

**Como:**
```go
// internal/metrics/savings.go
type SavingsEstimate struct {
    OriginalBytes   int
    CompactedBytes  int
    ReductionPct    float64
    ContextUsed     int     // from KiroCrew session
    EstimatedTokens int     // context_used / 4 (rough estimate)
}
```

**Dados disponíveis:**
- `context_used` em `~/.kiro/sessions/cli/<id>.json`
- Output original vs compactado (já medimos)

**Entrega:** `downshift stats` com savings estimado.

### 2. Model-Aware Routing Advisory (3 pts)

**O que:** Recomendar modelo baseado no tipo de comando e contexto.

**Como:**
```go
// internal/routing/advisory.go
func RecommendModel(cmd string, contextUsed int) string {
    // Comandos simples + contexto baixo → Haiku
    // Testes + contexto médio → Sonnet
    // Build complexo + contexto alto → manter atual
}
```

**Entrega:** Flag `--recommend-model` no compactor.

### 3. Metrics Dashboard (3 pts)

**O que:** Visualização de métricas acumuladas.

**Como:**
```bash
$ downshift dashboard
┌─────────────────────────────────────┐
│ Downshift Metrics (last 7 days)     │
├─────────────────────────────────────┤
│ Commands compacted: 847             │
│ Output reduced: 12.4 MB → 2.1 MB    │
│ Reduction: 83%                      │
│ Estimated tokens saved: ~3.1M       │
│ Model distribution:                 │
│   claude-sonnet-4: 72%              │
│   claude-opus-4: 18%                │
│   claude-haiku-4: 10%               │
└─────────────────────────────────────┘
```

**Entrega:** `downshift dashboard` ou `downshift stats --week`.

### 4. preToolUse Hook Integration (5 pts)

**O que:** Hook nativo no KiroCrew para métricas em tempo real.

**Como:**
```json
// ~/.kiro/hooks/downshift-metrics.json
{
  "name": "Downshift Metrics",
  "when": { "type": "preToolUse" },
  "then": {
    "type": "runScript",
    "script": "downshift hook --pre"
  }
}
```

**Entrega:** `downshift hook --install` para setup automático.

## Dependências

| Feature | Depende de |
|---------|-----------|
| Savings | context_used (já disponível) |
| Routing | Model detection (v0.1.4 ✅) |
| Dashboard | Exec-events (já temos) |
| Hook | preToolUse (F5 documentado) |

## Timeline

```
v0.1.4 (atual) ─────────────────────────────────────────────
  ✅ KiroCrew harness detection
  ✅ Model inference
  ✅ Compactor steering

v0.2.0 (próximo) ───────────────────────────────────────────
  🔲 Savings estimation
  🔲 Model advisory
  🔲 Metrics dashboard

v0.3.0 (futuro) ────────────────────────────────────────────
  🔲 preToolUse hook
  🔲 Real-time metrics
  🔲 Cost projection
```

## Não Planejado (bloqueado pelo KiroCrew)

| Feature | Bloqueio |
|---------|----------|
| Token counting real | ACP não expõe tokens |
| Cost calculation real | Sem tokens, sem custo |
| Auto model switching | Sem API para trocar modelo |

## Métricas de Sucesso

- Savings estimation dentro de 20% do real (quando tokens forem expostos)
- Dashboard útil sem tokens reais
- Hook instalável em < 1 minuto
