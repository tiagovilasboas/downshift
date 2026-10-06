# 09 · Decisões Técnicas e Racional de Arquitetura

O design do Downshift é pautado pelo equilíbrio deliberado entre requisitos de produto e restrições de engenharia em ambientes locais de desenvolvimento.

---

## Matriz de Decisões de Arquitetura

| Decisão de Arquitetura | O que o Produto Ganha | Qual é a Contrapartida de Engenharia |
|---|---|---|
| **Binário único em Go puro** | Execução ultraveloz do hook em sub-2ms; instalação simples sem dependências de runtime. | Não se utiliza o ecossistema padrão de bibliotecas pesadas de ML em Python no caminho crítico. |
| **Heurísticas de Regex como 1ª camada** | Decisões determinísticas, explicáveis e facilmente auditáveis. | O classificador depende de correspondência de vocabulário e padrões explícitos. |
| **Desempate prioriza capacidade superior** | Segurança contra subdimensionamento: tarefas no limiar nunca caem em modelos fracos demais. | Um empate duvidoso pode manter um custo ligeiramente mais alto. |
| **Tarefas `SIMPLE` no tier Small** | Economia máxima: ajustes pontuais e componentes isolados rodam no modelo econômico (Haiku/Luna). | Um ajuste `SIMPLE` incomumente complexo pode exigir que o modelo gaste mais turnos. |
| **Exigência de margem de confiança** | Evita oscilações de roteamento em tarefas ambíguas ou vagas. | Uma tarefa `MEDIUM` sem sinais fortes permanece no modelo atual (mesmo em `UPSHIFT`). |
| **Camada Semântica MiniLM local** | No holdout queimado (`benchmark/minilm-holdout.json`) a linha neural faz 96,3% de acurácia de tier e 0% de `FRONTIER → MID`, sem custo de rede. Não supera a rede de regex nesse arquivo. | Treino e sincronização de centróides requerem curadoria contínua de datasets. |
| **Sem chamadas a serviços remotos (Jev/APIs)** | Zero latência de rede, custo marginal zero na classificação e privacidade absoluta. | Não se utiliza modelos de raciocínio de ponta apenas para triagem. |
| **Separação entre Catálogo e Permissões** | Impossibilita erros onde o subagente falharia por tentar usar um modelo não contratado pela conta. | Exige manutenção do arquivo de permissões da sessão (`session-models.json`). |
| **Distinção entre Emissão e Aplicação** | Transparência nos relatórios: a telemetria não assume falsamente que o cliente acatou a instrução. | A confirmação final de uso depende do ecossistema do harness. |
| **Honor inferido na mesma sessão** | Codex já conta o follow-up sem reescrever o log. | Claude sem `session_id` não entra nessa conta. |
| **Graphify via comando** | Dá para ligar um grafo depois, fail-open. | Não é socket MCP nativo. |
| **Zero-Leakage no log de eventos** | Logs seguros para compartilhamento público e relatórios de auditoria. | Não é possível recuperar o texto original do prompt a partir dos arquivos de telemetria. |
| **Candidato softmax só em shadow** | Dá para comparar um candidato com rótulos revisados antes de qualquer promoção. | `DOWNSHIFT_SHADOW_WEIGHTS` não altera o hook. Sucesso sem `--required-tier` não vira treino. Ver [10 · Candidato em shadow](10-candidato-shadow.md). |

---

## O Caminho até a Versão 1.0 (Saída do Beta)

Para consolidar a graduação formal para a versão 1.0 (conforme mapeado em [`docs/BETA-EXIT.md`](../BETA-EXIT.md)), os passos finais são puramente empíricos:
1. **Acúmulo de Eventos com Custo Real (P3.4 e P3.5):** Atingir pelo menos 50 eventos com telemetria de tokens via `PostToolUse` no Claude Code para reportar a economia real em dólares.
2. **Confirmação em Produção (P1.3):** Registrar a confirmação visual na interface do Claude Code demonstrando o executor filho rodando no modelo econômico reescrito. O Codex (P1.5) já tem evidência em 2026-10-02.
3. **Evidência Multi-Ambiente (P2.3):** Incorporar dados consolidados anônimos de 2 ou mais desenvolvedores utilizando o roteador em suas rotinas diárias.
