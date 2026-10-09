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
* No Antigravity desktop 2.21.1, em 2026-10-09, um filho foi correlacionado ao argumento reescrito `flash_lite`; os metadados nativos do executor e da geração registraram `gemini-3.5-flash-lite` ([evidência e QA](../evidence/antigravity-executor-ack.md)). É uma observação real desse build; a associação entre picker, bolsão e alias continua pendente.
* No Codex, em 2026-10-02, o spawn seguinte da mesma sessão chegou com `requested_model` igual ao `final_model` anterior ([evidência](../evidence/codex-rewrite-honored-2026-10-02.md)). `downshift stats --export` conta isso em `rewrite_honored_inferred` sem reescrever o JSONL.
* No Claude Code, `PostToolUse` observa o modelo resolvido no lançamento. Quando o lançamento assíncrono não traz tokens, `SubagentStop` lê o transcript do filho, deduplica mensagens e vincula consumo à decisão. O [audit de custo](../evidence/billing-gap-audit.md) separa registros de uso, vínculos válidos e preços calculados; não equivale a uma fatura.

### Disponibilidade e cota

A precedência é lista do hook, export nativo configurado do Cursor, cache de
descoberta e arquivo do operador. O export precisa ser completo e recente;
arquivo configurado inválido segura a reescrita. A ordem do picker não mede
capacidade: essa fonte exige metadados de tier do resolver e preserva o piso da
tarefa. Cota e disponibilidade são requisitos separados. Uma porcentagem de uso
descreve sua janela ou bolsão, sem determinar quais modelos pertencem a ele.
Veja [cota](../quota.md) e [tarefas pendentes](../gap-tasks.md).

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Desenvolver um adapter cirúrgico por harness evita a dependência de um protocolo unificado inexistente no mercado e garante compatibilidade nativa com o ecossistema real de cada ferramenta.
* **Custo:** A maturidade dos hooks varia de acordo com o fornecedor. Por exemplo, contas gratuitas do Claude Code não possuem a ferramenta `Task`, enquanto versões legadas do Cursor ignoram alterações silenciosamente no campo de modelo.
