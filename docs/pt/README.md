# Downshift em português

Documentação de arquitetura do `harness-downshift`, separada por domínio. O README da raiz continua sendo o guia de instalação. Aqui está o fluxo, as regras e as decisões.

| Doc | Domínio |
|---|---|
| [01-entrada](01-entrada.md) | O que chega no hook e o que não chega |
| [02-classificador](02-classificador.md) | Sinais, classes e o que é acerto |
| [03-marcha](03-marcha.md) | Tier, upshift, downshift, confiança |
| [04-fallbacks](04-fallbacks.md) | Fail-open, payload, sessão desconhecida |
| [05-catalogo-e-sessao](05-catalogo-e-sessao.md) | De onde sai o id do modelo |
| [06-harnesses](06-harnesses.md) | Claude Code, Codex, Cursor, Antigravity, KiroCrew |
| [07-minilm](07-minilm.md) | Decisão local, hash e MiniLM neural |
| [08-telemetria](08-telemetria.md) | O que é gravado e o que nunca é |
| [09-decisoes](09-decisoes.md) | Decisões técnicas e o que ficou de fora |

Código de referência: `cmd/downshift`, `internal/core`, `internal/catalog`, `internal/semantic`, `internal/telemetry`.
