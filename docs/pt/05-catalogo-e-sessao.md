# 05 · Catálogo e Permissões de Sessão

O Downshift opera com uma distinção estrita entre duas fontes de informação: **o que existe no catálogo global** versus **o que a conta do desenvolvedor tem autorização para usar nesta sessão**.

---

## Perspectiva de Produto

* **O Catálogo (`catalog.json`):** Uma tabela de metadados de referência. Ele define quanto cada modelo custa por 1M de tokens, a qual família pertence e qual é o seu tier de capacidade (Small, Mid ou Frontier).
* **A Sessão Ativa (`session-models.json`):** O inventário dos IDs que aquela sessão pode selecionar. Quando o harness envia uma lista no payload, ela prevalece; caso contrário, o arquivo local é um snapshot mantido pelo operador, não uma descoberta automática.
* **Regra de Ouro:** O Downshift **nunca** injeta um modelo que não esteja no inventário da sessão. Os IDs aparecem no inventário porque são as escolhas reais do picker, mas nenhum nome de modelo é embutido na lógica de seleção.

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
3. **Seleção agnóstica de alvos:**
   * Cada lista deve estar ordenada por capacidade crescente (menos potente → mais potente).
   * O Downshift usa apenas IDs listados: pequenas tarefas partem do primeiro, tarefas médias do ponto central, tarefas de fronteira do último; upshift seleciona o mais potente elegível.
   * Preços, nomes, famílias e IDs do catálogo não escolhem nem substituem candidatos. O catálogo pode enriquecer metadados e manter compatibilidade com `explicit_only` conhecido.
   * O `session-models.json` ainda contém IDs, por necessidade: se o hook não fornece a lista do picker, é impossível saber quais modelos a sessão aceita sem uma integração nativa do harness. Lista ausente ou vazia continua sem rewrite.

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Evita quebras catastróficas onde um subagente falharia imediatamente porque a API do provedor recusou um ID de modelo que a conta não tem direito de acessar (*entitlement error*).
* **Custo:** Nos harnesses que não enviam o inventário do picker, o snapshot local precisa ser atualizado e ordenado pelo operador quando as opções da sessão mudarem. O catálogo não precisa conhecer um ID novo para que ele seja elegível.
