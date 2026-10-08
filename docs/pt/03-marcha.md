# 03 · Gestão de Marchas (Tiers e Roteamento)

A política de marcha (*gearbox*) compara o modelo que o harness usaria por padrão contra o tier mínimo necessário para atender à demanda com sucesso. A lógica conceitual é universal em todos os clientes (Claude Code, Cursor, Codex, Antigravity e KiroCrew); apenas os identificadores dos modelos variam.

---

## Perspectiva de Produto

| Complexidade | Tier Alvo | Ação se a sessão estiver em modelo econômico (Small) | Ação se a sessão estiver em modelo de ponta (Frontier) |
|---|---|---|---|
| **TRIVIAL** | **Small** | Mantém no modelo econômico | **DOWNSHIFT** (reduz para o modelo econômico) |
| **SIMPLE** | **Small** | Mantém no modelo econômico | **DOWNSHIFT** (reduz para o modelo econômico) |
| **MEDIUM** | **Mid** | **UPSHIFT** (eleva para o modelo intermediário) | **DOWNSHIFT** (reduz para o modelo intermediário) |
| **COMPLEX** | **Frontier** | **UPSHIFT** (eleva para o modelo de fronteira) | Mantém no modelo de fronteira |

### Mapeamento Concreto por Ambiente
* **Codex:** O modelo econômico é o `gpt-6-luna`, o intermediário é o `gpt-5.6-terra` e o modelo de ponta é o `gpt-6-sol`. Um componente Three.js simples roda no Luna; um export de CSV sobe para o Terra; uma refatoração arquitetural escala para o Sol.
* **Claude Code:** Econômico é o `claude-haiku-5-5`, intermediário é o `claude-sonnet-5-5` e o de ponta é o `claude-opus-5-5`.
* **Antigravity:** Econômico é o `flash_lite`, intermediário é o `flash` e o de ponta é o `pro`.

> **Decisão de Produto em `SIMPLE`:** Originalmente, tarefas `SIMPLE` eram direcionadas para o tier intermediário (`Mid`). Isso fazia com que inserções de campos ou pequenos ajustes de interface saíssem do Luna para o Terra (uma diferença de quase 25x no custo por token de entrada no catálogo). O ajuste de política rebaixou `SIMPLE` para o tier `Small`, preservando a economia máxima em tarefas pontuais.

---

## Detalhes de Engenharia

O mapeamento é orquestrado por `Complexity.Tier()` e `core.Route` em `internal/core/policy.go`:

1. **Vereditos Possíveis:**
   * `DOWNSHIFT`: O modelo atual é mais potente (e caro) do que a tarefa exige.
   * `OK`: O modelo atual é perfeitamente aderente ao tier necessário.
   * `UPSHIFT`: A tarefa requer mais capacidade cognitiva do que o modelo atual oferece.
   * `UNKNOWN`: O modelo atual da sessão não pôde ser determinado pelo payload.
2. **Freio de Confiança:** A função `ShouldRewriteModel` só autoriza a reescrita física do payload em transições de `DOWNSHIFT` ou `UPSHIFT` se o classificador apresentar **alta confiança** (vantagem de ≥ 2 pontos). Se houver dúvida estatística, o veredito é registrado na telemetria, mas o modelo ativo da sessão é preservado intacto.
3. **Modelos de Escolha Explícita (`explicit_only`):** Modelos marcados como `explicit_only` no catálogo (como `gpt-6-astra`) nunca são substituídos automaticamente pelo Downshift. Se o operador escolheu essa variante manualmente, sua intenção é soberana.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** A exigência de margem de confiança atua como salvaguarda simétrica. Uma tarefa `MEDIUM` duvidosa não gastará tokens do tier superior desnecessariamente.
* **Custo:** Se um modelo econômico (`Small`) se mostrar limitado para uma tarefa no limiar entre `SIMPLE` e `MEDIUM`, a política erra propositalmente pelo lado mais barato, exigindo que o desenvolvedor refine o pedido ou aumente a instrução caso a execução falhe.
