# Benchmark — Downshift × OpenCode (F2)

Data: 2026-10-18 · Binário: f2-bin (buildVersion=f2-e2e, sha b5648130…)
Dataset: `benchmark-opencode.json` (13 tarefas, estilo subagent spawn do OpenCode)

## 1. Classificador (downshift benchmark)

| Métrica | Valor |
|---|---|
| Total tasks | 13 |
| Complexity accuracy | 84.6% |
| Tier accuracy | 84.6% (CI95 61.5%–100%) |
| Frontier→small rate | 0% (seguro) |
| Frontier→mid rate | 33.3% (1/3 COMPLEX roteado para mid) |
| Small→mid rate | 14.3% (1/7 small roteado para mid, desperdício) |
| Small→frontier rate | 0% |

### Legacy vs CapabilityRouter v2 (--compare)
| Métrica | Legacy | v2 | Delta |
|---|---|---|---|
| Tier accuracy | 76.9% | 38.5% | -38.5% |
| Unsafe downgrade | 33.3% | 100.0% | +66.7% (FRONTIER→SMALL: 0→1; FRONTIER→MID: 1→2) |
| Over-routing | 0.0% | 0.0% | 0 |
| Risk-weighted loss | 0.692 | 2.308 | +1.615 |

**Recomendação do binário:** "No clear improvement — keep legacy as default. Collect more training data."
**Aviso:** menos de 5 exemplos por tier → exploratório. Dataset pequeno; não conclusivo.

## 2. Integração OpenCode (F2) — evidência coletada

| Passo | Resultado |
|---|---|
| Build f2-bin | OK (sha b5648130…) |
| Backup spike (da504ee0…) | OK |
| Install plugin + config.json 0600 | OK |
| reload #1 | OK ("Configuration reloaded") |
| plugin.setup | `code=runtime_ok` (carrega e valida) |
| Try-probe | route `big-pickle → deepseek-v4.1-flash-free` (exit 0) |
| C1 (bin inexistente) | `active:false code=bin_missing` ✓ |
| C2 (sha errado) | reload crashed (`ServiceUnavailableError`) — não exercitado; código trata checksum_mismatch no setup |
| Canonical run1 (H1–H5) | BLOQUEADO — rate limit no endpoint `opencode/*-free` |
| Negativo (sem marker) | BLOQUEADO — rate limit |
| C3 (stub malformado) | BLOQUEADO — rate limit |
| C4 (timeout) | não executado (opcional) |
| Restore (config canônica + reload) | OK |
| Marker final | ausente (dormente) |

## 3. Status do harness
OpenCode = **Beta** (owner decision 2026-10-18): adapter + route wire v1 + conformidade no repo; plugin de encaminhamento e observação live em desenvolvimento. Sem spawn live observado; sem reivindicação de produção.

## 4. Limitações / UNKNOWN
- E2E live não executado (rate limit do endpoint free + flakiness do reload do serviço).
- Dataset do benchmark pequeno (13 tarefas, <5 por tier) → métricas exploratórias.
- v2 CapabilityRouter underperforma Legacy neste dataset (38.5% vs 76.9%) — sinal, mas não conclusivo.
- Usage seam do OpenCode: NOT PROVEN (sem custo registrado).
