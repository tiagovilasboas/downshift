# Harnesses

A marcha é uma. A tomada muda.

**Produto.** Claude Code, Codex, Cursor, Antigravity e KiroCrew passam pelo mesmo julgamento de tamanho. O que muda é o nome do modelo e o momento em que o Downshift é chamado. Se a ferramenta não criar um subagente, nada acontece.

**Técnico.**

| Harness | Quando | Comando |
|---|---|---|
| Claude Code | ferramenta Task, antes e depois | `downshift claude-code`, `downshift claude-code-post-tool-use` |
| Codex | `Agent` ou `spawn_agent` | `downshift codex` |
| Cursor | spawn de subagente | `downshift cursor` |
| Antigravity | `invoke_subagent` | `downshift antigravity` |
| KiroCrew | preToolUse, sem `updated_input` | `downshift kirocrew` |

`rewrite_emitted` é emissão. No Codex, o sinal de que o harness obedeceu é o spawn seguinte da mesma sessão chegar com `requested_model` igual ao `final_model` anterior. PostToolUse no Claude Code só grava `usage` se o payload tiver `input_tokens` ou `output_tokens`. Nesta instalação, o Codex não aponta PostToolUse para o Downshift.

**Trade-off.** Um adapter por harness evita um protocolo único que nenhum vendor cumpre. O preço é evidência desigual: o Codex já mostrou a troca aplicada numa sessão local; o Claude Code, no plano free, nem tem a ferramenta Task.
