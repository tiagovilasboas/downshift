# Fase 6 — O que o downshift fecha sozinho

**8 pontos. Sem dependência externa. Sem esperar payload ao vivo, rate limit ou outro app.**

Tudo aqui é doc ou infra interna do repo: código, testes, CI. O downshift é o único dono.

---

## Lista de entrega

| Item | Pts | O que é | Dependência |
|------|-----|---------|-------------|
| F-2a | 1 | Arquivar `docs/architecture/project-architecture.md` (descreve tipos que não existem: `Session`, `Policy`, `ShouldEscalate`) ou reescrever alinhado ao código real. Remover referência do `harness-engineer.md`. | — |
| F-3b | 1 | Substituir links para `agentic-bdi.md` (arquivado) e `project-architecture.md` em `harness-engineer.md` e qualquer outro doc que ainda aponte para eles. | F-2a |
| F-4a | 1 | Corrigir `architecture.md`: contagem de arquivos de telemetria (31→40 non-test), e atualizar nota do import graph (existe `archtest.TestImportGraph`, não é "no test or CI step"). | — |
| F-4b | 1 | Atualizar a seção de compactação em `architecture.md`: citar `internal/shellwrap`, `internal/execwrap`, `AdviseAdapter` — já existem e o rewrite do CC está ativo. Remover referência a `downshift context compress` (não existe). | — |
| F-4c | 1 | Atualizar o grafo de imports ASCII em `architecture.md`: adicionar `accounting`, `execwrap`, `shellwrap`, `replay`, `api`, `cursorbridge`, `gateway` (todos existem na main hoje). | — |
| F-5a | 1 | Verificar se `internal/archtest.TestImportGraph` está na suite padrão (`go test ./...`) e se bloqueia imports proibidos (ex: `internal/core` importar `telemetry`). Reportar o que cobre e o que não cobre. | — |
| F-5b | 2 | Se o teste não bloqueia imports proibidos: adicionar casos específicos dos invariantes do `architecture.md` ao teste e garantir que o CI falhe quando a regra é violada. | F-5a |

**Total: 8 pts.**

---

## Ordem sugerida

F-4a + F-4b + F-4c podem ir em paralelo (tocam o mesmo arquivo, mas seções diferentes — staging seletivo por seção).

F-2a + F-3b em sequência (F-3b depende de F-2a para não quebrar link antes de arquivar).

F-5a antes de F-5b (F-5b é condicional ao achado de F-5a).

Todos cabem num PR único se forem pequenos; separar F-5b num PR próprio se virar gate de CI (mudança de comportamento).

---

## Critério de done

- `go test ./internal/docscheck/ ./internal/archtest/ ./internal/execwrap/ -count=1` verde.
- `go build ./...` verde.
- Nenhum link morto para `agentic-bdi.md` (arquivado) ou para tipos que não existem no código.
- `architecture.md` fiel ao `git ls-files internal/ | grep -v _test` contagem e ao grafo real.
- `TestImportGraph` cobre os invariantes do architecture.md e falha se violados.
