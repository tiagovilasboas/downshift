# 07 · Boost semântico por hash (em processo)

A triagem de complexidade roda dentro do binário. Não há chamada de API, subprocesso de embedding nem modelo neural no caminho do hook.

---

## O que o hash faz

O texto vira um vetor unitário de 384 dimensões com FNV-1a por palavra (`internal/semantic/hashembed.go`). Esse vetor é comparado por cosseno aos centróides em `internal/semantic/data/prototypes.json`.

Regras:

- só **sobe** a classe quando o regex está inseguro;
- não age em prompt sem sinal;
- não cria COMPLEX sozinho;
- `DOWNSHIFT_MINILM=0` desliga o boost.

Uma subida limpa a confiança e entra na telemetria como `SEMANTIC_BOOST`.

O comando externo MiniLM (`DOWNSHIFT_MINILM_EMBED`) e o arquivo `minilm.json` foram removidos em 2026-10-17. Números antigos de holdout neural não descrevem o binário atual.

Este boost entra no `core.Route`. O candidato softmax da v2 foi removido do binário; a direção atual está em [routing v2](../design/routing-v2.md).
