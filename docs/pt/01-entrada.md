# 01 · Entrada do Fluxo

O **Downshift** atua exclusivamente no momento exato em que um **subagente está prestes a ser instanciado**. Um chat comum de turno único, sem invocação de ferramentas de delegação, não passa pelo roteador.

---

## Perspectiva de Produto

* **Insumo Estrito:** O único dado processado é o texto descritivo da tarefa delegada ao subagente (`prompt`, `description`, `task` ou `message`).
* **Isolamento de Contexto:** Histórico acumulado da conversa, arquivos lidos anteriormente pelo agente principal e transcrições completas da sessão são totalmente ignorados.
* **Resultado Gerado:** Caso a reescrita seja aprovada pela política de segurança, o Downshift retorna a indicação de um modelo mais econômico (e, quando suportado pelo harness, o nível de esforço de raciocínio / *reasoning effort*). O Downshift **nunca** executa a tarefa nem gera código.

---

## Detalhes de Engenharia

O harness cliente invoca o binário localmente via `stdin`:

* **Claude Code:** `downshift claude-code` configurado como hook `PreToolUse` na ferramenta `Task`, e opcionalmente `downshift claude-code-post-tool-use` no `PostToolUse` para telemetria de consumo real de tokens.
* **Codex:** `downshift codex` configurado para as ferramentas `Agent` ou `spawn_agent` sob o protocolo `multi_agent_v2`.
* **Cursor:** `downshift cursor` interceptando requisições da ferramenta `Task`.
* **Antigravity:** `downshift antigravity` interceptando chamadas de `invoke_subagent`.
* **KiroCrew:** `downshift kirocrew` operando em modo de política e bloqueio orientativo (`exit 0` / `exit 2`).

### Limites e Proteções Operacionais
* **Teto de Carga (Max Payload):** Payloads via `stdin` com tamanho superior a 1 MB são rejeitados com `PAYLOAD_TOO_LARGE` e não passam por classificação. Em clientes como o Codex, quando a interface tenta enviar a árvore inteira da conversa, essa barreira impede o consumo excessivo de memória do processo.
* **Execução Rápida:** O comando de terminal `downshift try` executa o mesmo pipeline de `core.Route` sem o overhead de payload do harness.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Analisar apenas o pedido da tarefa mantém o processamento ultrarrápido (sub-2ms), o log estritamente anônimo e elimina o vazamento de código proprietário.
* **Custo:** A perda do histórico global pode reduzir a contextualização em prompts extremamente curtos (ex: "faça um jogo 3D" sem detalhes pode soar como uma alteração simples em um primeiro momento).
