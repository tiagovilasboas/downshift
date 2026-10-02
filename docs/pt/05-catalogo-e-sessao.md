# Catálogo e sessão

O catálogo (`internal/catalog/catalog.json`) não é a lista do que a conta pode abrir. Ele diz, para um id conhecido, o tier, a família, o effort e o preço de lista por 1M tokens.

A sessão é a lista do que este spawn pode receber. A ordem de leitura está em `docs/session-models.md`:

1. `session_models` ou `available_models` no payload, se o campo existir.
2. Senão `~/.harness-downshift/session-models.json`.
3. Se não houver lista, a sessão é desconhecida e o hook não troca o modelo.

O Codex, o Claude Code e o Cursor não mandam o picker inteiro. Mandam o modelo atual. Por isso um `-m gpt-5.4` recusado pela conta ChatGPT morre antes do hook: o Downshift não escolhe o modelo pai e não consulta a API da OpenAI.

Quando a troca é permitida, o id escrito é um membro da sessão. Se o id do catálogo não estiver na sessão, upshift usa o mais forte da lista e downshift o mais barato, nunca um id inventado.
