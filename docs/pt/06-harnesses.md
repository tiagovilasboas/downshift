# 06 · Adapters e Suporte a Harnesses

O motor de decisão do Downshift é único e agnóstico a provedores. No entanto, cada ferramenta cliente (*harness*) possui um protocolo de comunicação, formato de hook e momento de interceptação próprios.

---

## Perspectiva de Produto

O Downshift atua como uma ponte universal: a lógica que decide que "renomear uma variável é uma tarefa mecânica de tier Small" é rigorosamente idêntica no Claude Code, Cursor, Codex ou Antigravity. O que muda é a sintaxe do canal de entrada e o envelope de resposta que cada ferramenta exige.

---

## Detalhes de Engenharia

| Harness | Momento do Ciclo | Comando Executado | Mecanismo de Controle |
|---|---|---|---|
| **Claude Code** | Antes e após o spawn da ferramenta `Task` | `downshift claude-code`<br>`downshift claude-code-post-tool-use` | Hook `PreToolUse` com reescrita via `updatedInput.model`. Telemetria de uso via `PostToolUse`. |
| **Codex** | Spawn sob `multi_agent_v2` (`Agent` / `spawn_agent`) | `downshift codex` | Injeção de `model` e `reasoning_effort` no payload antes da submissão. |
| **Cursor** | Hook de interceptação da ferramenta `Task` | `downshift cursor` | Reescrita de `updated_input.model` em planos Pro/Ultra. |
| **Antigravity** | Interceptação de `invoke_subagent` | `downshift antigravity` | Modificação de argumentos via envelope `overwrite` em `Subagents[0].Model`. |
| **KiroCrew** | Hook binário em `preToolUse` | `downshift kirocrew` | **Modo Política:** Retorna `exit 0` para liberar ou `exit 2` para bloquear e instruir o agente a recriar o subagente no tier adequado. |

### Validação de Reescrita (`rewrite_emitted` vs Honrado)
* O status `rewrite_emitted` no log de eventos comprova que o Downshift interceptou a requisição e devolveu o payload reescrito para o harness.
* No Codex, em 2026-10-02, o spawn seguinte da mesma sessão chegou com `requested_model` igual ao `final_model` anterior ([evidência](../evidence/codex-rewrite-honored-2026-10-02.md)). `downshift stats --export` conta isso em `rewrite_honored_inferred` sem reescrever o JSONL.
* No Claude Code, o hook `PostToolUse` captura métricas reais de consumo (`input_tokens`, `output_tokens`) emitidas pelo provedor após a finalização da sub-tarefa.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Desenvolver um adapter cirúrgico por harness evita a dependência de um protocolo unificado inexistente no mercado e garante compatibilidade nativa com o ecossistema real de cada ferramenta.
* **Custo:** A maturidade dos hooks varia de acordo com o fornecedor. Por exemplo, contas gratuitas do Claude Code não possuem a ferramenta `Task`, enquanto versões legadas do Cursor ignoram alterações silenciosamente no campo de modelo.
