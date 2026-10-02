# 04 · Fallbacks e Tolerância a Falhas

O princípio fundamental de confiabilidade do Downshift é a **falha aberta (*fail-open*)**: aconteça o que acontecer, a inicialização do subagente jamais pode ser travada pelo roteador. O Downshift nunca bloqueia o fluxo do desenvolvedor para tentar "pensar melhor".

---

## Perspectiva de Produto

* **Continuidade Operacional:** Em qualquer situação de dúvida, erro de leitura ou incompatibilidade de dados, o subagente é executado normalmente com o modelo original herdado da sessão.
* **Impacto da Falha:** O desenvolvedor não perde a execução do seu comando; o único impacto é a perda temporária da oportunidade de otimização de custo (ou da elevação automática para um modelo superior).

---

## Detalhes de Engenharia

O ecossistema trata cenários de borda sem recorrer a chamadas externas a LLMs:

| Situação de Exceção | Comportamento do Downshift | Consequência |
|---|---|---|
| **JSON malformado no stdin** | Registra erro `INVALID_EVENT` na telemetria | O hook libera o spawn sem alterações (`allow`) |
| **Payload acima de 1 MB** | Emite `PAYLOAD_TOO_LARGE` | O processo não tenta desserializar o buffer |
| **Timeout de leitura no stdin** | Emite `INPUT_TIMEOUT` | Libera a execução imediatamente |
| **Sessão sem lista de modelos** | Sessão tratada como desconhecida (`unknown`) | Não reescreve o modelo |
| **Modelo recomendado fora da sessão** | Modelo não consta no allowlist ativo | Preserva o modelo original da sessão |
| **Transição com baixa confiança** | Margem de pontuação menor que 2 | Mantém o modelo da sessão inalterado |
| **Falha do embedder externo (MiniLM)** | Fallback instantâneo para centróides de hash local | Classificação segue com embeddings locais |
| **Falha no cálculo de embeddings** | Fallback para as heurísticas de regex | Classificação concluída com sinais estáticos |
| **Modo de controle (`DOWNSHIFT_NO_ROUTE=1`)** | Avalia a rota, gera evento `baseline`, mas não altera | Permite testes comparativos A/B |

---

## Compensações Técnicas (Trade-offs)

* **Vantagem:** Resiliência absoluta. O desenvolvedor nunca tem seu trabalho interrompido por falhas transitórias do Downshift.
* **Custo:** Um evento que exceda limites (como payloads gigantescos com transcripts inteiros) deixa de ser otimizado e roda no modelo padrão, podendo ter um custo mais elevado na fatura daquele turno específico.
