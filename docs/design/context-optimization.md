# Native Context Compressor & Context Sensor

Downshift includes an in-process, zero-dependency **Native Context Compressor** (`internal/compressor`) and event-driven **Context Sensor** (`internal/sensor`) to govern token consumption without altering critical diagnostics or introducing external runtimes.

---

## 1. Architectural Principles

1. **Zero External Runtime**: Written purely in Go, with zero Python, Rust, Node, network or LLM dependencies. `context enable` persiste o provider nativo e o contrato de exit code. O PostToolUse do Claude Code grava `CompressExit` em modo observe, com o exit code real, só quando essa flag está ligada. O hook não substitui a saída da ferramenta e não comprime prompt.
2. **Fowler Harness Taxonomy (Guia vs. Sensor)**:
   - **Guia (Execution / Transformation)**: Canal ativo de reescrita e execução nativo, ligado por padrão (`"exec": false` ou `DOWNSHIFT_EXEC=0` desliga) (`downshift compact`, `internal/execwrap`). Intervém apenas em comandos estritamente allowlistados (`go test`, `pytest`, `vitest`, `jest`, `playwright`, `cypress run`, `next build`, `phpunit`, `gradle`, `./gradlew`, `cargo test`, `tsc`, `git status`) a menos que o usuário o desligue.
   - **Sensor (Observability)**: Canal puramente passivo (`internal/sensor`, `PostToolUse`). Registra apenas agregados de bytes, classificações de formato e contadores de execução. Nunca altera a saída e nunca persiste comandos.
3. **Clean Code, SRP & Dependency Rule**: O `internal/compressor` é uma biblioteca pura (leaf package) sem dependências externas ou conhecimento de harnesses. O `sensor` observa métricas sem executar comandos; o `execwrap` orquestra subprocessos e gates de buffer sem alterar regras de negócio de harnesses.
4. **Deterministic & Fail-Open**: Compacta sucesso repetido só com exit code 0. **Preserva erro, stack trace, exit diferente de zero, linha que não seja `ok`/`passed`, busca, lista de arquivos e `git log`.**
5. **Observability vs. Transformability**: Distinguishes observational telemetry (e.g. `PostToolUse` telemetry) from transformational filters (command pipes or tool proxies).
6. **Epistemic Precision**: Metrics clearly distinguish **Observed** counts from **Estimated** heuristics and **Unavailable** harness channels.
7. **Privacy First**: Sensor stores only session hashes, format classifications, and byte aggregates. **Prompts, private code, and sensitive outputs are never persisted.**

---

## 2. Supported Formats & Compression Rules

| Format | Detection Heuristic | Safe Action | Fail-Open Guarantee |
|---|---|---|---|
| `go test` | `ok` package lines | Summarizes a suite only when every non-empty line is `ok` | Any other line or `FAIL` is preserved verbatim |
| `go test` (non-zero exit, `downshift compact` only) | A failed run: `--- FAIL`, `FAIL`, `panic:`, `[build failed]`, `[setup failed]`, `WARNING: DATA RACE` | Cuts only `ok <pkg>` lines (no coverage), `--- PASS` lines and the `=== RUN/PAUSE/CONT/NAME` headers of tests that passed; adds one `Go test: ok: N packages` line | Every other line stays byte-identical and in order. Raw on `-json`, CRLF, no final newline, stderr output or no failure signal; exit code unchanged. Other families still keep a non-zero exit raw |
| `go test -v` | Complete single-package `=== RUN` / `--- PASS` transcript and final `PASS` summary | Drops test names and timings only when there is no test output, then emits test counts | Any failure, test log output, incomplete or unrecognized line, stderr, or non-zero exit is preserved verbatim |
| `pytest` | `=== N passed in X.XXs ===`, test progress dots/names | Summarizes clean suite to `pytest: PASS (N passed in X.XXs)` with package/module overview | Compacted only when every line is a recognised plain pytest line and there is one passing summary; any other line (a failure, traceback, warning, stderr, or an unknown line) or a non-zero exit is preserved verbatim |
| `vitest` | `Test Files N passed`, `Tests N passed`, `✓` checks | Summarizes clean suite to `vitest: PASS (N test files, M passed in X.XXs)` | Any `FAIL`, `Error:`, unhandled rejection, assertion diff, stderr output, or non-zero exit is preserved verbatim |
| `jest` | `Test Suites` and `Tests` totals | Summarizes only matching complete pass totals, preserving skipped and todo counts | Failure markers, mismatched or missing totals, stderr output, or non-zero exit is preserved verbatim |
| `playwright` | `Running N tests using M workers`, `N passed (X.XXs)`, browser run lines (`[chromium]`, `[firefox]`, `[webkit]`) | Summarizes clean suite to `playwright: PASS (N passed across M workers in X.XXs)` | Any `failed`, `timed out`, `Error:`, assertion diff, trace attachment, retry warning, stderr output, or non-zero exit is preserved verbatim |
| `cypress run` | Completed `(Run Finished)` block with consistent Tests/Passing/Failing counts | Summarizes a complete run with passing, pending, skipped counts, spec, and duration | Errors, warnings, failed tests, incomplete or inconsistent totals, stderr output, or non-zero exit is preserved verbatim |
| `next build` | Complete Next.js build phases, static page generation, route tables, and shared chunk summary | Summarizes a recognized complete build with route counts by router, shared JS size, and build duration | Warnings, errors, unknown lines, incomplete generation/tables, stderr output, or non-zero exit is preserved verbatim |
| `phpunit` | Default dot progress, final test/assertion totals and optional runtime details | Summarizes only a complete all-pass run with matching progress and test counts | Failure, warning, test output, alternate printer, incomplete or inconsistent totals, stderr output, or non-zero exit is preserved verbatim |
| `flutter test`, `dart test` | Dart test reporter progress segments (`00:03 +12 ~1: name`, joined by carriage returns in the compact reporter) ending in `All tests passed!` | Summarizes only a run where every segment is a progress update with no failure counter and non-decreasing counters, to `dart/flutter test: PASS (N passed, M skipped in MM:SS)` | Failure counters, `[E]`, `Some tests failed`, `Expected:`/`Actual:`, test print output, pub get or lock chatter, warnings, ANSI colour (`dart test` colours by default; `--no-color` compacts), CRLF, a missing final newline, any unknown line, stderr output, or non-zero exit is preserved verbatim. `flutter analyze` / `dart analyze` are not wrapped: a clean run prints one short line |
| `mvn` / `mvnw` / `./mvnw` (`test`, `verify`, `package`, `install`, `clean`) | Single-module Maven 3.9 transcript of known `[INFO]` lines only: goal headers, resource/compile chatter, Surefire per-class and total lines, `BUILD SUCCESS`, `Total time` | One line per goal executed, plus the Surefire totals line, `BUILD SUCCESS` and `Total time` verbatim | Any `[WARNING]`/`[ERROR]`, `BUILD FAILURE`, skipped tests, failures, test stdout, download lines, reactor builds, unknown lines, CRLF, ANSI colour, missing trailing newline, stderr output, or non-zero exit is preserved verbatim |
| `gradle` / `./gradlew` | `> Task :module:name` lines (UP-TO-DATE, NO-SOURCE, FROM-CACHE, SKIPPED), daemon startup line, `BUILD SUCCESSFUL in X`, `N actionable tasks: ...` | Collapses the task lines into one count line; keeps `BUILD SUCCESSFUL` and the actionable-tasks line verbatim | `FAILED`, `FAILURE:`, `w:`/`e:` Kotlin lines, `warning:`, deprecation notices, `Note:`, test output, stack traces, unknown lines, ANSI, CRLF, missing final newline, stderr output, or non-zero exit is preserved verbatim |
| `cargo test` | `running N tests`, `test result: ok. N passed; 0 failed`, `finished in X.XXs` | Summarizes clean suite to `cargo test: PASS (N passed; 0 failed in X.XXs)` | Any `FAILED`, `error[E...]`, `warning:`, `panicked at`, doc-test failure, stderr output, or non-zero exit is preserved verbatim |
| `tsc` | Clean exit with zero errors (empty output or `Found 0 errors.`) with exit code 0 | Summarizes clean typecheck to `tsc: PASS (0 errors)` | Any `error TS\d+:`, diagnostic message, syntax error, semantic error, stderr output, or non-zero exit is preserved verbatim |
| `git status` | `On branch`, `nothing to commit` | Compacts a clean status | Preserves branch, conflicts and uncommitted changes |
| `git log` | `commit [0-9a-f]{40}` | Preserved verbatim | A summary would drop commit bodies |
| `search` (grep/rg) | `path:line: content` | Preserved verbatim | A prefix would hide matches |
| `file_listing` | `find`, `ls -R`, tree structures | Preserved verbatim | A prefix would hide names |
| `logs` | Repeating log line prefixes | Collapses consecutive identical lines (`[Repeated N times]`) | Preserves first and last timestamp + all error logs |

### Adding a family

A new `downshift compact` family ships all of the following; the conformance suite is the gate.

1. **Match rule**: a `Family` constant and its `FamilyOf` case in `internal/execwrap/policy.go`, plus its `expectedFormat` entry in `internal/execwrap/run.go`.
2. **Detector**: a `Format` constant (`internal/compressor/types.go`) and its branch in `detectFormat`; failures and warnings must set the error flag so the output stays raw.
3. **Compressor**: the format's branch in `applyFormatCompression`, summarising only an output that is provably clean.
4. **Fixtures**: at least one clean and one preserve fixture in `internal/compressor/testdata/corpus/`, each classified in `testdata/corpus/MANIFEST` as `<clean|preserve> <format> <file>`.

The gate needs no edit when a family is added. `go test ./internal/compressor ./internal/execwrap` checks, for every fixture in the MANIFEST: a clean fixture compacts as its format and gets strictly shorter; a preserve fixture comes back byte-identical; and for each deterministic mutation of a clean fixture (unknown line, no final newline, `\r`, ANSI colour, truncation, duplicated summary, an injected `error`/`FAIL`/`warning`/`panic` line) the result is byte-identical to the mutated input or keeps every danger line the mutation added. The execwrap registry test fails with the missing piece when a `Family` has no `expectedFormat` entry or no clean and preserve fixture. A known gap is listed in `knownViolations` in `conformance_test.go` with a reason and a TODO naming the family; do not use it for a new family.

---

## 3. Economia de tokens & Multi-Turn Cache Inflation

Não há medição reproduzível de economia de tokens neste repositório. `downshift context benchmark` mede bytes em fixtures versionadas (original bytes, reduced bytes, byte savings). A unidade é bytes. Custo de roteamento e acurácia saem como `not measured`. Isso não é token e não é dólar. `downshift context metrics` pode contar comandos no log de compactação; os campos de token ficam `unavailable` e não são uma economia medida.

### Os Campeões de Queima de Tokens de Output

Três formatos destacam-se como fontes críticas de queima descontrolada de tokens e inflação de contexto em sessões de agentes autônomos:

1. **Playwright (Testes E2E Multi-Browser)**:
   - Suítes de 10 a 30 testes ponta a ponta executadas em múltiplos browsers (Chromium, Firefox, WebKit) disparam workers concorrentes.
   - Cada teste gera logs de inicialização de instâncias de browser, context switching, relatórios de passos (`[chromium] › auth.spec.ts:12:3 › login success`), tempos por worker e tabelas de resumo.
   - Uma execução limpa gera rotineiramente **3.000 a 5.000 tokens** de puro overhead de logs de aprovação.
   - A compactação para `playwright: PASS (24 passed across 3 workers in 12.4s)` corta **~99% dos bytes**, convertendo 4.000+ tokens em apenas ~25 tokens.

2. **Cargo Test (Ecossistema Rust)**:
   - Em projetos Rust, `cargo test` aciona a compilação incremental de crates de teste (`Compiling ...`, `Finished test ...`), descoberta de testes, listas de testes unitários (`test module::test_name ... ok`), testes de integração e doc-tests (`Doc-tests ...`).
   - Uma suíte média gera **1.000 a 2.500 tokens** em saídas que apenas confirmam que dezenas de asserções passaram sem pânico.
   - A compactação para `cargo test: PASS (42 passed; 0 failed in 2.15s)` reduz a carga para ~20 tokens (**98%+ de economia de bytes**).

3. **TSC / TypeScript Typecheck (`tsc --noEmit`)**:
   - Em loops de refatoração dirigidos por agentes (TDD / lint-and-fix), o agente invoca repetidamente `tsc --noEmit` para verificar a integridade dos tipos.
   - Em caso de sucesso limpo, a ferramenta emite saída vazia ou `Found 0 errors.`. No entanto, em monorepos ou projetos com referências cruzadas (`--build`), comandos semânticos podem emitir diagnósticos informativos e listagens de projeto.
   - A sintetização determinística em `tsc: PASS (0 errors)` padroniza o feedback para menos de 5 tokens, eliminando ambiguidades e evitando alucinações causadas por saídas vazias ou parciais.

### O Mecanismo de Multi-Turn Cache Debt

Em agentes autônomos de engenharia (Claude Code, Cursor, Codex), o impacto dessas saídas de teste verbosas é multiplicativo:
1. **Turno de Execução ($T_0$)**: A saída do teste é gravada como saída de ferramenta e entra no contexto imediato como tokens de Fresh Input.
2. **Turnos Subsequentes ($T_1 \dots T_k$)**: O histórico do diálogo é re-lido em cada turno subsequente da sessão.
   - Uma execução de **Playwright** de 4.000 tokens no turno $T_2$ em uma sessão de 15 turnos gera $13 \times 4.000 = \mathbf{52.000}$ **tokens de Cache Read** acumulados.
   - Uma execução de **Cargo Test** de 2.000 tokens no turno $T_3$ gera $12 \times 2.000 = \mathbf{24.000}$ **tokens de Cache Read** acumulados.
   - Múltiplas validações de **TSC** ao longo da sessão evitam poluição residual de contexto turno a turno.
3. **Cache Write Invalidation**: Qualquer invalidação de cache (edição de arquivo de código, prompt do usuário ou expiração do TTL de 5 minutos do provedor LLM) força a regravação integral desses milhares de tokens como **Cache Write**, cobrado a até $3.75 por MTok no Sonnet 3.5.

Ao compactar essas suítes para resumos de ~20–25 tokens, a economia imediata de 95% a 99% em bytes corta na raiz a dívida de cache cumulativa, economizando de 50.000 a 100.000 tokens de cache por sessão complexa. A estimativa nos comandos de CLI é sempre reportada como heurística (`saved_bytes / 4`), nunca como contagem exata faturada.

---

## 4. Matriz de roteamento e tabela de bytes

`downshift context benchmark` lista os quatro cenários. Custo de roteamento e acurácia saem como `not measured`. Em seguida o comando roda o compressor em fixtures versionadas (`go test` só com `ok`, `go test` com linha que não é `ok`, logs repetidos, hits de busca) e imprime original bytes, reduced bytes e byte savings. A unidade é bytes. Não inventa tokens nem dólares.

| Cenário | Roteamento de Modelo | Compressão de Contexto | Fator de Custo | Redução de Tokens | Acurácia |
|---|---|---|:---:|:---:|:---:|
| **A. Baseline** (Sem Downshift) | Frontier (Unrouted) | Nenhuma (Raw logs) | not measured | not measured | not measured |
| **B. Downshift Only** | Roteamento Dinâmico | Nenhuma (Raw logs) | not measured | not measured | not measured |
| **C. Compressor Only** | Frontier (Unrouted) | Native Squelch | not measured | not measured | not measured |
| **D. Downshift + Compressor** | Roteamento Dinâmico | Native Squelch | not measured | not measured | not measured |

---

## 5. Modos de Operação

- **`off`**: Pass-through mode; retorna a saída completamente inalterada.
- **`observe`** (*Padrão de runtime*): Calcula métricas de redução e formato sem modificar a saída.
- **`safe`**: Compacta só com `--exit 0` e só quando o formato inteiro é reconhecido como sucesso. Sem exit code, a saída original é preservada. Exit diferente de zero também preserva.

---

## 6. Referência da CLI

```bash
# Verificar status da otimização de contexto (ativo por padrão)
downshift context status

# Listar providers e matriz de capacidades
downshift context providers

# Executar diagnóstico do compressor e harnesses
downshift context doctor

# Reativar o compressor nativo se tiver sido desativado
downshift context enable

# Desativar um companion registrado (o compressor nativo fica sempre ligado)
downshift context disable headroom

# Processar saída de ferramentas via stdin. Sem --exit 0, safe preserva a entrada.
cat verbose_test_output.txt | downshift context compress safe --exit 0

# Contadores do sensor. Tokens ficam unavailable.
downshift context metrics

# Bytes medidos nas fixtures. Custo de roteamento continua not measured.
downshift context benchmark
```

---

## 7. Matriz de Capacidades por Harness

| Harness | Canal de Telemetria | Canal de Transformação | Mecanismo |
|---|---|---|---|
| **Claude Code** | `PostToolUse` observa bytes e o exit code | O hook não reescreve a saída da ferramenta | Com a flag ligada, grava `CompressExit` em observe. O modelo continua vendo a saída original. Não comprime prompt. |
| **Cursor** | Nenhum hook reescreve saída de ferramenta | Não há substituição automática | Compressão só quando alguém chama o compressor |
| **Codex** | Nenhum hook reescreve saída de ferramenta | Não há filtro automático na sessão | Compressão só quando alguém chama o compressor |
| **Antigravity** | Nenhum hook reescreve saída de ferramenta | Não há filtro direto na sessão | Compressão só quando alguém chama o compressor |
| **KiroCrew** | Policy mode no spawn | O hook não reescreve a saída da ferramenta | `include_memory=false` no respawn é política de spawn, não compactação de prompt |

---

## 8. Providers e companions

`internal/contextopt` tem dois tipos de provider. O **nativo** roda no processo e é o default (seção 1). Um **companion** (`contextopt.Companion`) é uma ferramenta externa que o usuário instala e executa; o Downshift a detecta, registra que está em uso (`Config.Companion`) e aponta para o comando dela. Nunca instala, lança, faz `wrap` ou configura (uma integração anterior fazia `init` em nome do usuário e foi removida no #71).

| Provider | Tipo | Detecção | `enable` persiste | O que muda no hook |
|---|---|---|---|---|
| `native` | in-process | sempre presente | `provider=native`, contrato de exit code | PostToolUse grava `CompressExit` em observe |
| `headroom` | companion | `headroom` no PATH + `--version` | `companion=headroom` | nada; o proxy comprime antes da API, o hook vê a saída original |

`downshift context enable` registra um companion detectado por padrão. `doctor` mostra uma linha `companion` (ok quando registrado e presente; warn com o fix quando detectado e não registrado, ou registrado e ausente). Métricas do companion ficam `unavailable`: o Headroom tem os próprios contadores (`headroom savings`) e o Downshift não os lê. Guia de uso, tabela por harness e o que mais o Downshift conhece: [guides/companions.md](../guides/companions.md).

---

## 9. Requisitos de Telemetria sem Vazamento (Fullstack, AppSec, QA)

Para suportar suítes modernas como `pytest`, `vitest`, `playwright`, `cargo test` e `tsc` sem comprometer a segurança operacional, os requisitos de telemetria seguem governança rígida entre disciplinas:

### AppSec (Privacidade e Segurança)
- **Zero Prompt & Code Leakage**: O compressor e os hooks nunca capturam, armazenam ou transmitem prompts, código-fonte ou corpos de saída.
- **Zero Path Leakage na Telemetria**: `exec-events.jsonl` e `context-sensor.json` gravam estritamente metadados categorizados (`family`: `pytest` | `vitest` | `jest` | `playwright` | `cypress` | `next-build` | `phpunit` | `gradle` | `cargo-test` | `tsc` | `go-test` | `git-status`, `exit_class`, contagens brutas de `raw_bytes` e `emitted_bytes`, flag booleana `compacted`). **Caminhos de arquivos (`/Users/.../test_auth.py`, `tests/e2e/login.spec.ts`, `src/lib.rs`), flags CLI, variáveis de ambiente ou argumentos de testes jamais chegam ao disco de telemetria.**
- **Superfície Restrita de Reescrita**: O `execwrap.Decide` só considera comandos sob 512 bytes compostos exclusivamente por caracteres seguros (alfanuméricos, espaços e `- _ . / : = , + @ %`). Proíbe pipes, redirecionamentos, crases, aspas e substituição de comandos. Comandos suportados incluem chamadas canônicas como `jest`, `npx jest`, `playwright test`, `npx playwright test`, `cypress run`, `npx cypress run`, `next build`, `npx next build`, `phpunit`, `./vendor/bin/phpunit`, `cargo test`, `tsc` e `npx tsc`.
- **Limites de Buffering e DoS Protection**: Limite rígido de buffer em memória (`2 MiB`) para proteção contra streams torrenciais gerados por suítes E2E de Playwright ou compilações extensas do compilador Rust (`rustc`).
- **Recursion Guard**: Injeção da variável `DOWNSHIFT_EXEC_DEPTH=1` em processos-filhos para impedir aninhamento acidental ou loops de reescrita.
- **Isolamento de Processos**: Processos filhos rodam em seu próprio process group com encaminhamento transparente de sinais (`SIGTERM`, `SIGINT`, `SIGHUP`, `SIGQUIT`), assegurando encerramento limpo de processos pesados (como instâncias de browsers headless e processos de compilação) ao cancelar um turno.

### QA (Engenharia de Testes e Confiabilidade)
- **Corpus Adversarial e Fuzzing**: Conjunto abrangente de fixtures cobrindo:
  - `pytest`: variações `pytest-xdist`, flags `-v`, `-s`, warnings deprecados, doctests.
  - `vitest`: ANSI colors, barras de progresso, multi-threads, testes assíncronos rejeitados.
  - `jest`: totais de suites e testes completos, com casos de falha, snapshots, skipped e todo.
  - `go test -v`: transcript completo de pacote único, saída de `t.Log`, subtestes e blocos de falha.
  - `cypress run`: resumo completo, execução incompleta, contagens inconsistentes, erro no output e falha acionável.
  - `next build`: fases completas, tabela de rotas/chunks, warnings, tabelas truncadas e linhas desconhecidas.
  - `phpunit`: progresso completo de sucesso, falha acionável, warning, saída de teste e totais inconsistentes.
  - `playwright`: suítes com retries ativados, timeouts de locator, falhas em múltiplos browsers simultâneos, crashes de browser worker, e links para visualizadores de trace/HTML report.
  - `cargo test`: testes unitários misturados com doc-tests, warnings de compilação do rustc (`warning:`), erros de borrow-checker (`error[E...]`), panics com stack traces (`panicked at ...`).
  - `tsc`: erros encadeados de inferência de tipos (`error TS2322`), erros de sintaxe JSX/TSX, e projetos com referências compostas (`--build`).
- **Garantia Inviolável de Fail-Open**: Qualquer suite contendo marcadores de falha (`FAILED`, `ERROR`, `AssertionError`, `failed`, `timed out`, `error[E`, `error TS`, `panicked at`, `UnhandledPromiseRejection`), traceback, ou código de saída diferente de zero é emitida 100% bruta (verbatim). Testes de regressão garantem que qualquer diagnóstico quebrado nunca seja omitido ou sintetizado incorretamente.
- **Contrato de Saída Idêntica sob Erro**: Se o compressor estourar limite de memória (`2 MiB`), encontrar caracteres de controle inválidos ou receber bytes no stderr antes do término, o gate imediatamente desiste e faz streaming raw byte a byte.

### Fullstack (Experiência do Desenvolvedor e Integração)
- **Zero-Dependency Runtime**: Executado inteiramente no binário compilado em Go, sem exigir bibliotecas Python (`pytest-json-report`), pacotes npm (`vitest-custom-reporter`, reporters customizados do Playwright) ou crates adicionais do Rust.
- **Transparência de Compactação**: Adição padronizada do marcador `[downshift compact: compacted N to M bytes]` ao final da saída, permitindo ao desenvolvedor e ao LLM saberem exatamente que a visualização foi resumida de forma segura.
- **Kill-Switch Imediato**: Suporte a desligamento sem reinicialização de daemon via flag `--no-compact` ou variável de ambiente `DOWNSHIFT_EXEC=0`.

