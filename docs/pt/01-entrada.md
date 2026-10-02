# Entrada do fluxo

O Downshift não vê a conversa inteira. Ele vê o spawn de um subagente.

O harness (Claude Code, Codex, Cursor, Antigravity ou KiroCrew) chama o binário como hook, com um JSON no stdin. O comando depende do harness. No Claude Code é `downshift claude-code` antes da ferramenta Task e `downshift claude-code-post-tool-use` depois. No Codex é `downshift codex` no `PreToolUse` de `Agent` ou `spawn_agent`.

O adapter extrai só o texto da tarefa (`prompt`, `description`, `task` ou `message`, conforme o harness) e o modelo atual. Histórico, arquivos lidos e transcript não entram na classificação.

Se o JSON passar de 1 MB, o hook para com `PAYLOAD_TOO_LARGE` e não escolhe modelo. A interface do Codex, quando manda o fio inteiro da conversa, cai nesse teto. Um `downshift try "..." codex gpt-6-luna` no terminal usa o mesmo `core.Route` sem esse payload.

O que sai do hook, quando a troca é permitida, é um modelo que já existe na sessão e, se o harness tiver effort nativo, um esforço. O spawn segue. O Downshift não escreve o código da tarefa.
