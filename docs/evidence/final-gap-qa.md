# Gate final de QA — 2026-10-09

harness-downshift by Tiago de Carvalho Vilas Boas
https://github.com/tiagovilasboas/downshift

**PASS**, no escopo do código, sensores e evidências delimitadas deste patch.
Não há falha de software confirmada pendente neste gate. A coleta real do Cursor
continua **bloqueada**; este resultado não declara atuação automática com cota
real em todos os harnesses, conclusão do beta ou sucesso da CI remota.

Checkout avaliado: `codex/gap-evidence-tasks`, sobre
`827a688fb354b4411eb626d4ede21f62c1a7b3cb`, com as alterações de integração no
working tree. Ambiente local: macOS arm64, Go 1.27.1, Node 26.7.0,
Python 3.14.7 e Python do sistema 3.9.6. Os resultados abaixo antecedem o commit;
o SHA do commit final deve ser associado a este relatório na PR.

## Comandos e resultados

| Checagem executada | Resultado |
|---|---|
| `go test ./...` | Exit 0 no baseline; a suíte completa também foi exercitada pelo gate com race |
| `make test` → `go test -race -cover ./...` | Exit 0; nenhuma race reportada |
| `go vet ./...` | Exit 0 |
| `node --test web/app.test.cjs internal/cursorbridge/extension/decoder.test.cjs` | 6 testes, 6 PASS, 0 FAIL |
| `python3 -W error::ResourceWarning -m unittest discover -s scripts -p 'test_collect_antigravity_evidence.py' -v` | 12 testes, PASS, Python 3.14.7 |
| Mesmo comando com `/usr/bin/python3` | 12 testes, PASS, Python 3.9.6 |
| Build temporário de `./cmd/downshift` + `./examples/run-all.sh` com PATH/estado isolados | Exit 0; três fixtures de adaptadores |
| Replay pelo binário compilado, export Cursor simulado, sem lista manual | 4 casos PASS: fresh, exhausted, stale, malformed |
| `git diff --check` | Exit 0 |

O build e o replay usaram `tempfile.TemporaryDirectory`, arquivos de estado e
eventos temporários. Nenhum binário instalado, arquivo privado ou hook real foi
substituído. O replay exigiu rewrite para o modelo pequeno financiado e preservação
de um campo irmão; as três observações negativas exigiram ausência de rewrite.
Ele prova a integração do consumidor CLI com uma fixture, não coleta do provedor
nem execução de um filho pelo Cursor.

Trechos dos resultados locais:

```text
ok  github.com/tiagovilasboas/downshift/internal/core       coverage: 89.9% of statements
ok  github.com/tiagovilasboas/downshift/internal/telemetry  coverage: 88.3% of statements
Node: tests 6 / pass 6 / fail 0
Python 3.14.7: Ran 12 tests / OK
Python 3.9.6: Ran 12 tests / OK
OK: all adapter smoke checks passed
PASS: compiled Cursor CLI fixture replay: fresh/no manual list, exhausted, stale, malformed
```

## Riscos exercitados

| Caminho | Evidência do sensor computacional |
|---|---|
| Precedência e fechamento da cota | `internal/core/session_usage_test.go`: hook > export nativo > cache; 8 cenários impedem que fonte inferior saudável reabra crédito quando a superior está inválida, expirada ou esgotada |
| Disponibilidade nativa do Cursor | `internal/core/session_native_test.go`: precedência hook > export > descoberta > operador, IDs exatos, lista completa, tempo original, expiry/futuro, malformed/oversized, ausência e isolamento entre harnesses |
| Snapshot único | A rotação do arquivo após resolver disponibilidade não troca a observação de cota no mesmo planejamento |
| Piso e direção do modelo | Testes de sessão/tier e 3 casos independentes: tarefa pequena em modelo mid não sobe para frontier por falta de cota; troca mid mais cara em preço de saída é retida; mid mais barato continua elegível |
| Upshift explícito | 2 casos independentes exigem rewrite para mid regular financiado com flag desligada e frontier explícito financiado com flag ligada; não basta apenas excluir frontier |
| Scope e consumo | `internal/quota/quota_test.go` e adaptador Codex: janela compartilhada/secundária, pool exato, saldo esgotado, freshness e reset, contexto/crédito comprado não confundidos com cota, harness diferente retido |
| Contagem aplicada | API em `internal/server/status_counts_test.go` e renderer real em `web/app.test.cjs`: All=2 e Cursor=1 em fixture com verdicts retidos; KiroCrew vazio mantém zero e esconde economia |
| Vínculo de uso | `internal/telemetry/usage_linkage_test.go`: referência ausente/órfã, não decisão, harness/session conflitante, fora da janela, agente/evento repetido e agentes distintos |
| Baseline desconhecido | Adaptador Claude Code e export preservam preço dos tokens observados; baseline ausente não fabrica economia comparativa |
| Sensor Antigravity | 12 fixtures: cadeia exata, divergência executor/generation, rewrite ausente, protobuf truncado/overflow/oversized/ambíguo, JSON profundo, IDs inválidos, row bound, privacidade de erro/output e conexão somente leitura |

Os sensores de contagem aplicada incluem API e renderer com DOM mínimo; não houve
novo E2E de navegador, screenshot ou trace neste gate. O script histórico
`report-e2e/live-report.check.mjs` espera o antigo contador de verdicts, portanto
o PASS histórico não foi reutilizado como prova da semântica aplicada atual.

## Achados resolvidos e hipótese refutada

- **Minor, confirmado:** JSON profundamente aninhado produzia traceback no
  Python 3.9.6 porque `RecursionError` não era capturado pelo sensor manual.
  Repro: executar a suíte Python com `-k deeply_nested` antes da correção.
  O implementador corrigiu a fronteira de erro; o mesmo teste passou depois,
  com stderr sanitizado e stdout vazio. Confiança alta.
- **Major, identificado na integração:** filtrar pools podia transformar um
  verdict DOWNSHIFT em seleção de maior capacidade. O implementador adicionou
  o guard; os casos independentes de direção/piso acima passaram. QA não
  apresenta um teste anterior à correção como se o tivesse executado.
- **Minor documental:** a descrição do diagnóstico Cursor ainda tratava a
  divergência de precedência como atual; foi atualizada como achado histórico
  corrigido. Não afetava a execução.
- **Hipótese refutada:** a exclusão de modelo explícito não elimina o fallback
  financiado regular; o teste forte exige o rewrite correto nos dois estados
  da flag. Nenhuma correção de produção foi necessária para essa hipótese.

## Aceites de evidência

| Critério | Resultado e limite |
|---|---|
| GAP-AG-HONOR / P5.5 | PASS para o filho observado no Antigravity 2.21.1: original `pro`, aplicado `flash_lite`, referência nativa exata e executor + 10 registros de geração `gemini-3.5-flash-lite`. São observações de um único filho. [Evidência](antigravity-executor-ack.md) |
| GAP-USAGE-COUNT | PASS para o contrato corrigido: 15 registros brutos e 5 vínculos resolvidos; não são 15 vínculos. Deduplicação é do contador vinculado; agregados históricos de custo mantêm semântica de registros. [Auditoria](billing-gap-audit.md) |
| GAP-COST-NONZERO / P3.5 técnico | PASS: $0.424866 de economia estimada por tokens/preços de catálogo; 5/5 vínculos conferem tokens/modelo nativos, incluindo 2/2 positivos. Baseline reutiliza os mesmos tokens como contrafactual; não mede uma segunda execução nem fatura. Dashboard não é requisito desse cálculo. [Auditoria](billing-gap-audit.md) |
| GAP-REPORT-COUNTS | PASS para API/renderer testados: apenas shifts aplicados entram no contador/economia |
| GAP-CURSOR-AVAILABILITY | PASS de código/fixtures e replay CLI isolado; não fecha a fonte real, autoload no hook instalado ou executor acknowledgement |

QA inspecionou os relatórios sanitizados, o sensor e suas fixtures. A aquisição
dos bancos nativos Antigravity e a auditoria dos cinco transcripts foram realizadas
pelos responsáveis indicados nas evidências; QA não releu bancos/transcripts
privados nem gerou novo turno pago para repetir essas observações.

## Pendências externas preservadas

O [diagnóstico Cursor](cursor-gap-diagnostic.md) documenta que Cursor 3.24.9
restringe `vscode.cursor` a extensões built-in. A tentativa falhou em `transport`,
antes dos RPCs; nenhum export foi produzido. A extensão temporária foi removida
e sua ausência conferida pelo responsável. SOURCE, AUTOLOAD e HONOR continuam
bloqueados ou pendentes; não há prova de consumo deste código pelo hook instalado.

Continuam abertos: associação estrutural alias/modelo/pool no Antigravity,
consumidores reais da cota Codex/Claude Code, P3.4 (5 vínculos validados < 50),
controle/período P3.7, dados independentes, generalização e revalidações trimestrais.
O preço de saída do catálogo orienta desempate/guard; isso não promete menor custo
total para qualquer mix de tokens. Veja [fila de tarefas](../gap-tasks.md).

Guia inferencial: contratos de fonte, escopo, piso e aceites em `docs/quota.md`,
`docs/session-models.md`, `docs/stats-export.md` e na fila de tarefas. Sensores
computacionais: gates offline, regressões Go/Node/Python e telemetria local;
sensor inferencial: esta avaliação dos limites de evidência. Eixos: behaviour,
architecture fitness e maintainability. O workflow contém os comandos Go,
Node 22 e Python 3.12; isso não prova que uma CI remota já executou. Nenhum
pre-commit ou novo coletor contínuo é alegado.

**Veredito: PASS no escopo acima, com as pendências de runtime preservadas.**
