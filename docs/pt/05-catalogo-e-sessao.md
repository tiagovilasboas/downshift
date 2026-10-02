# Catálogo e sessão

Há duas listas, e elas não são a mesma coisa.

**Produto.** O catálogo diz quanto um modelo custa na tabela e se ele é pequeno, médio ou forte. A sessão diz o que a sua conta, neste momento, pode realmente usar. O Downshift só escreve um nome que a sessão já conhece. Ele não pergunta à OpenAI ou à Anthropic o que você tem direito de abrir.

**Técnico.** `internal/catalog/catalog.json` traz id, família, effort e preço de lista por 1M. A sessão vem, nesta ordem, de `session_models` ou `available_models` no payload, senão de `~/.harness-downshift/session-models.json`. Se não houver lista, a sessão é desconhecida e não há troca (`docs/session-models.md`). Codex, Claude Code e Cursor mandam o modelo atual, não o picker. Upshift, quando o id do catálogo não está na sessão, usa o mais forte da lista. Downshift usa o mais barato.

**Trade-off.** Não inventar modelo evita um id que o harness recusa. O custo: se o processo pai do Codex já morreu porque a conta ChatGPT não aceita `gpt-6-luna`, o hook nem chega a rodar. Isso não é um bug da marcha. É entitlement da conta.
