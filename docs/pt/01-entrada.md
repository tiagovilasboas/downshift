# Entrada do fluxo

O Downshift só entra quando um subagente vai nascer. Um chat comum, sem ferramenta de spawn, não passa por ele. Por isso um “pong” escrito na própria conversa não aparece no relatório.

**Produto.** O pedido do subagente é o insumo. O histórico da conversa, os arquivos já lidos e o transcript não entram. O que sai, se a troca for permitida, é outro modelo e, às vezes, outro esforço de raciocínio. O Downshift não implementa a tarefa.

**Técnico.** O harness chama o binário no stdin. Claude Code: `downshift claude-code` no PreToolUse da ferramenta Task, e `downshift claude-code-post-tool-use` depois. Codex: `downshift codex` em `Agent` ou `spawn_agent`. O adapter lê `prompt`, `description`, `task` ou `message` e o modelo atual. JSON acima de 1 MB vira `PAYLOAD_TOO_LARGE` e não classifica: a interface do Codex, quando manda o fio inteiro, cai nesse teto. `downshift try` usa o mesmo `core.Route` sem esse payload.

**Trade-off.** Classificar só o pedido deixa o log limpo e barato. O custo é perder contexto: um jogo 3D descrito em uma frase pode parecer simples, e uma conversa longa no Codex pode nem chegar à classificação.
