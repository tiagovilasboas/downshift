# Harnesses

A marcha é a mesma. Muda o adapter e o nome do modelo.

| Harness | Quando o hook roda | Comando |
|---|---|---|
| Claude Code | ferramenta Task, antes e depois | `downshift claude-code` e `downshift claude-code-post-tool-use` |
| Codex | `Agent` ou `spawn_agent` | `downshift codex` |
| Cursor | spawn de subagente | `downshift cursor` |
| Antigravity | `invoke_subagent` | `downshift antigravity` |
| KiroCrew | preToolUse, sem `updated_input` | `downshift kirocrew` |

`rewrite_emitted` significa que o hook imprimiu a troca. Não significa que o executor usou esse modelo. No Codex, a evidência mais próxima é o spawn seguinte da mesma sessão chegar com o `requested_model` igual ao `final_model` anterior.

O PostToolUse do Claude Code grava `outcome: "usage"` só se o payload trouxer `input_tokens` ou `output_tokens`. Sem isso não há dólar real. O Codex, nesta instalação, não tem PostToolUse apontando para o Downshift.
