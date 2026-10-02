# 05 · Catálogo e Permissões de Sessão

O Downshift opera com uma distinção estrita entre duas fontes de informação: **o que existe no catálogo global** versus **o que a conta do desenvolvedor tem autorização para usar nesta sessão**.

---

## Perspectiva de Produto

* **O Catálogo (`catalog.json`):** Uma tabela de metadados de referência. Ele define quanto cada modelo custa por 1M de tokens, a qual família pertence e qual é o seu tier de capacidade (Small, Mid ou Frontier).
* **A Sessão Ativa (`session-models.json`):** O conjunto real de modelos autorizados e disponíveis para a conta do usuário naquele exato momento.
* **Regra de Ouro:** O Downshift **nunca** injeta um modelo que a sessão não possua. Ele não consulta APIs remotas da OpenAI ou Anthropic para adivinhar planos de assinatura; se um modelo não estiver na lista autorizada da sessão, ele não será escolhido como destino de reescrita.

---

## Detalhes de Engenharia

1. **Catálogo Embarcado e Sobrescrita:**
   * O catálogo canônico fica embutido no binário (`internal/catalog/catalog.json`).
   * Operadores podem estender ou substituir dados colocando um arquivo em `~/.harness-downshift/catalog.json`.
2. **Resolução da Lista de Sessão:**
   A descoberta dos modelos autorizados segue uma ordem estrita de precedência:
   1. Campos `session_models` ou `available_models` fornecidos diretamente no payload do hook pelo harness (se presente, essa lista é soberana).
   2. Configuração local por ID de sessão em `~/.harness-downshift/session-models.json` (`sessions.<harness>.<session_id>`).
   3. Configuração geral por harness em `~/.harness-downshift/session-models.json` (ex: chaves `"claude-code"`, `"cursor"`, `"codex"`, `"antigravity"`).
   4. Se nenhuma lista for localizada, a sessão é declarada como desconhecida (`Known = false`) e o Downshift falha aberto sem realizar reescritas.
3. **Seleção de Alvos em Cenários de Incompatibilidade:**
   * Se o modelo sugerido pelo catálogo não constar na lista da sessão:
     * Em um **upshift**, o sistema seleciona o modelo mais potente entre os que estão disponíveis na sessão.
     * Em um **downshift**, o sistema seleciona o modelo mais barato entre os disponíveis.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Evita quebras catastróficas onde um subagente falharia imediatamente porque a API do provedor recusou um ID de modelo que a conta não tem direito de acessar (*entitlement error*).
* **Custo:** Se o usuário possui acesso a um novo modelo lançado recentemente, mas não atualizou seu `session-models.json` ou catálogo local, o Downshift não utilizará esse novo ID até que ele seja incluído na configuração.
