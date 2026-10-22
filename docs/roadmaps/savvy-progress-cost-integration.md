# Roadmap: Integrar $ (custo) no painel savvy-progress

> **Status:** Draft  
> **Data:** 2026-10-22  
> **Contexto:** O savvy-progress mostra "Tokens saved" via `downshift session --panel`, mas não usa o PriceBook/formula do accounting v1 para calcular o $ economizado.

## Objetivo

Expor `cost_saved_estimate` (USD) no JSON do `session --panel`, calculado pela fórmula T3:
```
C_actual   = Σ price(model_i, tokens_i)
C_baseline = price(requested_model, Σ tokens_i)
Saved      = C_baseline − C_actual
```

## Estado atual

```
savvy-progress → downshift session --panel → telemetry.PanelView
                                            ├── tokens_saved (contagem bruta)
                                            └── compactor tokens estimate
                                                    ↓
                                            tokens_saved_estimate = routing + compactor
```

O PriceBook e formula existem em `internal/accounting/formula/` mas não são chamados pelo panel.

## Tarefas

### T1: Expor PriceBook no session panel (2 SP)

**Arquivo:** `cmd/downshift/sessionpanel.go`

1. Carregar o catálogo embedado via `catalog.Embedded()`
2. Criar `catalog.NewPriceBook(cat)`
3. Coletar buckets de tokens dos eventos da sessão
4. Chamar `formula.Saved(requestedModel, buckets, pb)`

**Dependência:** Precisa de um "requested model" da sessão. Usar o primeiro `FromModel` dos eventos ou um default.

### T2: Adicionar campos de custo no panelOut (1 SP)

**Arquivo:** `cmd/downshift/sessionpanel.go`

```go
type panelOut struct {
    telemetry.PanelView
    Compactor           *compactorPanel `json:"compactor,omitempty"`
    TokensSavedEstimate int64           `json:"tokens_saved_estimate"`
    
    // Novo: custo via formula T3
    CostSaved           *costPanel      `json:"cost_saved,omitempty"`
}

type costPanel struct {
    ActualCostUSD   float64 `json:"actual_cost_usd"`
    BaselineCostUSD float64 `json:"baseline_cost_usd"`
    SavedUSD        float64 `json:"saved_usd"`
    SavedPercent    float64 `json:"saved_percent"`
    Quality         string  `json:"quality"` // exact, mixed, partial, unpriced
}
```

### T3: Converter eventos telemetry para TokenBucket (2 SP)

**Arquivo:** `internal/accounting/session.go` (ou novo `internal/accounting/panel.go`)

```go
func EventsToTokenBuckets(events []telemetry.Event) []formula.TokenBucket {
    // Filtrar eventos de usage
    // Agrupar por modelo
    // Retornar buckets
}
```

### T4: Testes (2 SP)

1. `sessionpanel_test.go`: verificar que `cost_saved` aparece no JSON
2. `formula_test.go`: já existe, confirmar cobertura com dados do panel
3. Teste de integração: sessão real → panel → verificar valores

### T5: Atualizar savvy-progress (1 SP)

**Arquivo:** `mods/savvy-progress/hooks/register.tsx`

1. Ler `cost_saved.saved_usd` do JSON
2. Mostrar como tile alternativo ou tooltip
3. Manter "Tokens saved" como principal (decisão owner 2026-10-19)

**Opcional:** Toggle para mostrar $ em vez de tokens

### T6: Documentação (1 SP)

1. Atualizar `mods/savvy-progress/README.md`
2. Atualizar `docs/accounting.md` (se existir)
3. Adicionar caveat do D4 (~15% bias em downshifts reais)

## Estimativa total

| Tarefa | SP | Dependência |
|--------|:--:|-------------|
| T1: PriceBook no panel | 2 | — |
| T2: Campos no panelOut | 1 | T1 |
| T3: EventsToTokenBuckets | 2 | — |
| T4: Testes | 2 | T1, T2, T3 |
| T5: Atualizar savvy-progress | 1 | T1, T2 |
| T6: Documentação | 1 | T5 |
| **Total** | **9** | |

## Decisões pendentes

1. **Qual é o "requested model" da sessão?**
   - Opção A: Primeiro `FromModel` dos eventos
   - Opção B: Modelo mais frequente
   - Opção C: Campo explícito no session start (não existe ainda)
   - **Recomendação:** Opção A (simples, conservador)

2. **Mostrar $ por default ou tokens?**
   - Decisão owner 2026-10-19: tokens only
   - **Recomendação:** Manter tokens, $ como tooltip/hover ou flag opt-in

3. **Incluir Compactor saving no $ ?**
   - Compactor salva bytes → tokens, não $ direto
   - Converter para $ seria: `compactor_tokens * avg_price_per_token`
   - **Recomendação:** Não incluir na v1, manter separado

## Riscos e caveats

1. **Bias de +15% (D4):** Downshifts geram mais tokens, inflando o baseline. Documentar.
2. **Modelos unpriced:** Alguns modelos não têm preço no catálogo. Quality = "partial" nesses casos.
3. **Catálogo stale:** O catalog.json embedado pode ficar desatualizado. Já tem processo de atualização.

## Ordem de execução sugerida

```
T3 (buckets) → T1 (PriceBook) → T2 (campos) → T4 (testes) → T5 (savvy) → T6 (docs)
     ↑              ↑
     └──────────────┴── podem rodar em paralelo
```

## Fora de escopo

- [ ] Compactor saving em $ (future work)
- [ ] Dashboard de custo histórico (usar `downshift stats`)
- [ ] Alertas de custo (future work)
- [ ] Atualização dinâmica de catálogo (já existe, não precisa mudar)
