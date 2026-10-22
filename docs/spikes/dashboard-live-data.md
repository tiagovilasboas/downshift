# Spike: Dashboard Live Data Integration

**Status:** 🟡 Planned  
**Created:** 2026-10-22  
**Author:** Tiago Vilas Boas  

## Objective

Integrar dados reais do accounting layer no dashboard, substituindo dados mock.

## Context

O dashboard foi simplificado (PR #446) e usa a API `/api/sessions`. 
O accounting layer v1 está implementado em `internal/accounting/`.
Próximo passo: conectar os dois.

## Roadmap

| Phase | Story Points | Description | Status |
|-------|--------------|-------------|--------|
| P1: API endpoint refactor | 2 | `/api/sessions` usar accounting.GlobalSummary | 🟡 Todo |
| P2: Real-time updates | 2 | WebSocket ou SSE para painel live | 🟡 Todo |
| P3: Session drill-down | 3 | Endpoint `/api/session/:id` com detalhes completos | 🟡 Todo |
| P4: Compactor integration | 2 | Métricas do execwrap no painel | 🟡 Todo |
| P5: Cost calculation | 2 | Usar formula package para USD savings | 🟡 Todo |

**Total: 11 story points**

## Dependencies

- `internal/accounting/` (✅ done)
- Dashboard v2 (✅ done, PR #446)
- PriceBook em `internal/accounting/formula/` (✅ done)

## Acceptance Criteria

- [ ] Dashboard mostra dados reais do gateway
- [ ] Tokens saved calculados via accounting.Saved()
- [ ] Compactor stats do execwrap
- [ ] Refresh automático a cada 3s (já implementado no frontend)
- [ ] Sem mock data em produção
