# Guia de Roteamento e Delegação de Modelos por Subagente

> Downshift por Tiago Vilas Boas  
> Repositório oficial: https://github.com/tiagovilasboas/downshift  
> Licença: Apache 2.0

O **Downshift** atua como uma camada determinística de infraestrutura para delegação e roteamento de modelos em agentes de codificação. Ele garante que cada subagente dispare no modelo e esforço ideais para a sua responsabilidade, respeitando rigorosamente as políticas declaradas pelo desenvolvedor.

---

## 1. Filosofia de Delegação (Decisão de Design O8)

O princípio fundamental do roteamento no Downshift é:
> **O orquestrador escolhe, o Downshift aplica, a sua política delimita e o Downshift mede.**

Ao contrário de abordagens frágeis que colocam outro LLM para rotear chamadas (gerando latência, custos e incertezas), o Downshift segue premissas sólidas de engenharia:

- **Zero Chamadas a LLMs no Caminho Crítico:** O código do Downshift é 100% Go nativo e determinístico. Uma decisão de roteamento leva menos de 2 milissegundos na CPU local.
- **Fail-Open por Padrão:** Qualquer dúvida na interpretação da tarefa, ausência de metadados ou falha no payload mantém inalterado o modelo original solicitado pelo orquestrador.
- **Preservação do Cache de Prompt:** A sessão principal de chat do desenvolvedor nunca é alterada no meio de um turno. O roteamento atua exclusivamente ao instanciar novos subagentes, garantindo que o cache de prompt da conversa principal permaneça 100% íntegro.

---

## 2. Como Funciona a Intercepção via Hooks

Quando o seu harness de desenvolvimento (como Claude Code, Codex ou Cursor) cria um subagente, o Downshift intercepta a chamada através de um gancho de ciclo de vida nativo (`PreToolUse`):

```text
[Sessão Líder / Orquestrador]
         │
         │ Dispara subagente (ex: modelo 'frontier', tarefa de lint simples)
         ▼
[Hook PreToolUse do Downshift]
   1. Lê modelo, tipo de agente e metadados
   2. Avalia contra policy.json e catálogo
   3. Aplica ordem de precedência estrita
   4. Reescreve modelo e reasoning_effort se autorizado
         │
         ▼
[Harness executa o subagente no modelo adequado (ex: 'small')]
```

Apenas os atributos `model` e `reasoning_effort` são passados pela reescrita; todos os demais parâmetros do subagente (como ferramentas liberadas, diretório de trabalho e instruções) são preservados de maneira idêntica.

---

## 3. Ordem Estrita de Precedência

O Downshift avalia cada solicitação de subagente através de uma esteira ordenada de prioridades. **A primeira regra que der match encerra a avaliação e define a ação:**

| Ordem | Nível de Avaliação | Comportamento e Efeito |
|:---:|---|---|
| **1** | **Modelo Fixado (*Pinned*)** | Se o orquestrador ou o usuário fixou um modelo específico, ele é **mantido intacto**. |
| **2** | **Perfil Declarado (*Profile*)** | Se houver um perfil em `policy.json` que case com a tag ou tipo do subagente, suas regras são aplicadas. |
| **3** | **Regra Ordenada Declarada** | A primeira regra customizada do `policy.json` que casar com tipo, esforço ou limites de cota vence. |
| **4** | **Dica de Tier do Orquestrador** | Se o orquestrador sugeriu `[tier=small|mid|frontier]`, o Downshift move o subagente para baixo até o tier indicado. |
| **5** | **Classificação da Tarefa (*Task Class*)** | Se a tarefa for classificada com alta certeza como `TRIVIAL` ou `SIMPLE` e **não pertencer a categorias de risco**, ela desce para `small`. Tarefas `MEDIUM` ou `COMPLEX` nunca descem por esta regra. |
| **6** | **Modelo Solicitado Mantido (*Fallback*)** | Nenhuma regra casou ou houve incerteza: o modelo solicitado originalmente é mantido na íntegra. |

---

## 4. Guardrails e Salvaguardas de Segurança

Para garantir a confiabilidade do ambiente de produção, o Downshift implementa salvaguardas nativas:

### Dúvida Mantém o Modelo (*Doubt Keeps*)
Se o classificador tiver baixa confiança na complexidade da tarefa, ou se o prompt for ambíguo, o modelo de maior capacidade solicitado pelo orquestrador é rigorosamente mantido. Economia de tokens nunca pode justificar redução da qualidade de entrega.

### Proteção Contra Elevação Não Autorizada
Por padrão, o Downshift opera apenas no sentido descendente (*downshift*): ele nunca eleva uma solicitação de modelo menor para um modelo mais caro por inferência autônoma.  
A única exceção é a flag opt-in `routing.raise_complex: true` no `policy.json`, que permite que uma tarefa classificada decisivamente como `COMPLEX` pedida em `small` suba no máximo para `mid`.

### Proteção de Modelos Explícitos (*Explicit Only*)
Modelos no catálogo marcados como `routing: "explicit_only"` (geralmente variantes de fronteira com raciocínio ultra-profundo e altíssimo custo) nunca são injetados pelo Downshift de forma automática.

### Pisos de Tier por Projeto (*Tier Floors*)
Você pode declarar em seu repositório pisos mínimos de modelo para impedir que tarefas críticas (como módulos de pagamento, autenticação, criptografia ou infraestrutura) sejam executadas em modelos compactos.

---

## 5. Configuração do `policy.json`

O arquivo `.downshift/policy.json` na raiz do seu repositório (ou em `~/.downshift/policy.json` como padrão global) permite customizar o comportamento de roteamento.

Para inicializar um modelo padrão:

```bash
downshift policy init
```

Para validar a integridade semântica e sintática do seu arquivo:

```bash
downshift policy check
```

### Exemplo Completo de Configuração:

```json
{
  "$schema": "https://raw.githubusercontent.com/tiagovilasboas/downshift/main/schemas/policy.schema.json",
  "routing": {
    "by_class": true,
    "raise_complex": false
  },
  "tier_floors": {
    "security": "frontier",
    "payment": "frontier",
    "database_migration": "mid"
  },
  "profiles": {
    "fast-tests": {
      "model": "small",
      "reasoning_effort": "low"
    },
    "architecture-review": {
      "model": "frontier",
      "reasoning_effort": "high"
    }
  },
  "rules": [
    {
      "name": "Mechanical research to small",
      "match": {
        "agent": "researcher",
        "task_class": ["TRIVIAL", "SIMPLE"]
      },
      "action": {
        "tier": "small"
      }
    },
    {
      "name": "Keep security audits on frontier",
      "match": {
        "agent": "security"
      },
      "action": {
        "tier": "frontier"
      }
    }
  ]
}
```

---

## 6. Observabilidade e Diagnóstico de Decisões

O Downshift oferece transparência total sobre o que acontece nos bastidores.

### Explicando Decisões com `downshift session --why`
Execute a qualquer momento para ver em linguagem natural clara o motivo de cada spawn ter sido alterado ou mantido:

```bash
downshift session --why
```

Exemplo de saída:
```text
Spawn #3 (agent: tester):
  - Solicitado: claude-3-7-sonnet (mid)
  - Decisão: alterado para claude-3-5-haiku (small)
  - Motivo: Tarefa classificada como TRIVIAL ("rodar testes unitários em internal/calc") sem riscos detectados.
  - Regra aplicada: Task Class (verdict: "class").
```

### Estatísticas e Relatórios de Economia
- `downshift stats`: Exibe o resumo quantitativo de spawns roteados, mantidos e estimativas de tokens poupados.
- `downshift report`: Detalha tokens consumidos por tarefa aceita e histórico por harness.
- `downshift watch`: Painel de terminal interativo ao vivo acompanhando cada chamada em tempo real.

---

## 7. Pausa Temporária do Roteamento

Se você estiver conduzindo um experimento específico e quiser desativar temporariamente o roteamento da sessão atual sem desinstalar ou alterar arquivos:

```bash
# Pausar o roteamento para a sessão corrente (dura 12 horas ou até o comando resume)
downshift session --pause

# Reativar o roteamento imediatamente
downshift session --resume
```

Enquanto a sessão estiver pausada, 100% dos subagentes serão criados exatamente com os modelos solicitados pelo orquestrador, operando como um grupo de controle natural.

---

## 8. Integração via Ferramentas MCP

Ao executar `downshift mcp serve`, o Downshift expõe um conjunto de 13 ferramentas nativas para os orquestradores de IA:

- `providers`: Lista os modelos suportados e disponíveis no harness atual.
- `status`: Informa a integridade de cotas, janelas de consumo e histórico da sessão.
- `apply`: Simula qual tier e modelo seriam aplicados para determinada tarefa antes do disparo.
- `plan`: Valida planos de delegação em lote.
- `playbook`: Consulta instruções padronizadas de trabalho para subagentes em frentes de Backend, Frontend e Arquitetura.
- `delegate`, `collect`, `cancel`, `diff`, `send`, `steer`, `release`: Primitivas avançadas para ciclo de vida de workers isolados em worktrees de Git.
