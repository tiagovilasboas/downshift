# 01 · Entrada do Fluxo

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

O **Downshift** atua exclusivamente no momento exato em que um **subagente está prestes a ser instanciado**. Um chat comum de turno único, sem invocação de ferramentas de delegação, não passa pelo roteador.

---

## Perspectiva de Produto

* **Tarefa mais fatos locais:** O classificador lê a tarefa delegada; o roteador também usa modelo de referência, modelos e metadados da sessão, capacidades do harness, cota local e política do projeto.
* **Contexto delimitado:** O histórico não vira entrada de classificação. No Claude Code, o adapter pode ler a cauda do transcript do pai para identificar o modelo de referência. O transcript do filho é lido depois, no `SubagentStop`, para uso e verificações observadas.
* **Resultado:** O plano pode manter, reduzir ou elevar o tier conforme risco, confiança e política. Ele retorna um modelo elegível na sessão e esforço quando suportado. O harness aplica a resposta e executa a tarefa.

---

## Detalhes de Engenharia

O harness cliente invoca o binário localmente via `stdin`:

* **Claude Code:** `downshift claude-code` no `PreToolUse` com matcher `Agent|Task`; `claude-code-post-tool-use` observa o modelo resolvido; `claude-code-subagent-stop` lê uso e verificações no transcript do filho.
* **Codex:** `downshift codex` para `spawn_agent` sob o protocolo `multi_agent_v2`; aplicação da reescrita é inferida, e uso do filho não é registrado.
* **Cursor:** `downshift cursor` interceptando requisições da ferramenta `Task`.
* **Antigravity:** `downshift antigravity` interceptando chamadas de `invoke_subagent`.
* **KiroCrew:** `downshift kirocrew` operando em modo de política e bloqueio orientativo (`exit 0` / `exit 2`).

### Limites e Proteções Operacionais
* **Teto de Carga (Max Payload):** Payloads via `stdin` com tamanho superior a 1 MB são rejeitados com `PAYLOAD_TOO_LARGE` e não passam por classificação. Em clientes como o Codex, quando a interface tenta enviar a árvore inteira da conversa, essa barreira impede o consumo excessivo de memória do processo.
* **Execução Rápida:** O comando de terminal `downshift route` executa o mesmo pipeline de `core.Route` sem o overhead de payload do harness.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Classificação local e limites de payload evitam uma chamada adicional de LLM. O log contém metadados, sem texto da tarefa; o tempo depende do tamanho da entrada. Isso não substitui os controles de privacidade do harness e do provedor.
* **Custo:** A perda do histórico global pode reduzir a contextualização em prompts extremamente curtos (ex: "faça um jogo 3D" sem detalhes pode soar como uma alteração simples em um primeiro momento).
