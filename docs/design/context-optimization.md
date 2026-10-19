# Native Context Compressor & Context Sensor

Downshift includes an in-process, zero-dependency **Native Context Compressor** (`internal/compressor`) and event-driven **Context Sensor** (`internal/sensor`) to govern token consumption without altering critical diagnostics or introducing external runtimes.

---

## 1. Architectural Principles

1. **Zero External Runtime**: Written purely in Go, with zero Python, Rust, Node, network or LLM dependencies. `context enable` persiste o provider nativo e o contrato de exit code. O PostToolUse do Claude Code grava `CompressExit` em modo observe, com o exit code real, só quando essa flag está ligada. O hook não substitui a saída da ferramenta e não comprime prompt.
2. **Fowler Harness Taxonomy (Guia vs. Sensor)**:
   - **Guia (Execution / Transformation)**: Canal ativo de reescrita e execução opt-in (`downshift exec`, `internal/execwrap`). Intervém apenas em comandos estritamente allowlistados (`go test`, `pytest`, `vitest`, `git status`) quando habilitado pelo usuário.
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
| `go test` | `ok` package lines | Summarizes a suite only when every non-empty line is `ok` | Any other line, `FAIL`, or non-zero exit is preserved verbatim |
| `pytest` | `=== N passed in X.XXs ===`, test progress dots/names | Summarizes clean suite to `pytest: PASS (N passed in X.XXs)` with package/module overview | Any `FAILED`, `ERROR`, `AssertionError`, traceback, warning flag, stderr output, or non-zero exit is preserved verbatim |
| `vitest` | `Test Files N passed`, `Tests N passed`, `✓` checks | Summarizes clean suite to `vitest: PASS (N test files, M passed in X.XXs)` | Any `FAIL`, `Error:`, unhandled rejection, assertion diff, stderr output, or non-zero exit is preserved verbatim |
| `git status` | `On branch`, `nothing to commit` | Compacts a clean status | Preserves branch, conflicts and uncommitted changes |
| `git log` | `commit [0-9a-f]{40}` | Preserved verbatim | A summary would drop commit bodies |
| `search` (grep/rg) | `path:line: content` | Preserved verbatim | A prefix would hide matches |
| `file_listing` | `find`, `ls -R`, tree structures | Preserved verbatim | A prefix would hide names |
| `logs` | Repeating log line prefixes | Collapses consecutive identical lines (`[Repeated N times]`) | Preserves first and last timestamp + all error logs |

---

## 3. Economia de tokens & Multi-Turn Cache Inflation

Não há medição reproduzível de economia de tokens neste repositório. `downshift context benchmark` mede bytes em fixtures versionadas (original bytes, reduced bytes, byte savings). A unidade é bytes. Custo de roteamento e acurácia saem como `not measured`. Isso não é token e não é dólar. `downshift context metrics` pode contar comandos no log de compactação; os campos de token ficam `unavailable` e não são uma economia medida.

### O Mecanismo de Multi-Turn Cache Debt

Em agentes autônomos de engenharia (Claude Code, Cursor, Codex), o impacto de saídas de teste verbosas (como suítes de 50+ testes em `pytest` ou `vitest` despejando centenas de linhas e 2.000 a 4.000 tokens) é multiplicativo:
1. **Turno de Execução ($T_0$)**: A saída do teste é gravada como saída de ferramenta e entra no contexto.
2. **Turnos Subsequentes ($T_1 \dots T_k$)**: O histórico do diálogo é re-lido em cada turno subsequente da sessão. Um dump de teste de 2.500 tokens lido por mais 15 turnos gera **$15 \times 2.500 = 37.500$ tokens de Cache Read**.
3. **Cache Write Invalidation**: Qualquer invalidação de cache (mudança de arquivo, prompt de usuário ou expiração de TTL de 5 minutos) força a regravação desses milhares de tokens como **Cache Write**, inflando o custo em até $3.75 por MTok no Sonnet 3.5.

Ao compactar uma suíte de 50+ testes aprovados para um resumo de ~20 tokens (`pytest: PASS (52 passed in 1.4s)`), o corte de 95%+ em bytes evita a dívida de cache cumulativa em toda a vida útil da sessão. A estimativa nos comandos de CLI é sempre reportada como heurística (`saved_bytes / 4`), nunca como contagem exata faturada.

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

# Desativar a qualquer momento (opt-out persistido)
downshift context disable

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

`internal/contextopt` tem dois tipos de provider. O **nativo** roda no processo e é o default (seção 1). Um **companion** (`contextopt.Companion`) é uma ferramenta externa que o usuário instala e executa; o Downshift a detecta, registra que está em uso (`Config.Companion`) e aponta para o comando dela. Nunca instala, lança, faz `wrap` ou configura (a integração RTK fazia `init` em nome do usuário e foi removida no #71).

| Provider | Tipo | Detecção | `enable` persiste | O que muda no hook |
|---|---|---|---|---|
| `native` | in-process | sempre presente | `provider=native`, contrato de exit code | PostToolUse grava `CompressExit` em observe |
| `headroom` | companion | `headroom` no PATH + `--version` | `companion=headroom` | nada; o proxy comprime antes da API, o hook vê a saída original |

`downshift context enable` registra um companion detectado por padrão. `doctor` mostra uma linha `companion` (ok quando registrado e presente; warn com o fix quando detectado e não registrado, ou registrado e ausente). Métricas do companion ficam `unavailable`: o Headroom tem os próprios contadores (`headroom savings`) e o Downshift não os lê. Guia de uso, tabela por harness e o que mais o Downshift conhece: [guides/companions.md](../guides/companions.md).

---

## 9. Requisitos de Telemetria sem Vazamento (Fullstack, AppSec, QA)

Para suportar suítes modernas como `pytest` e `vitest` sem comprometer a segurança operacional, os requisitos de telemetria seguem governança rígida entre disciplinas:

### AppSec (Privacidade e Segurança)
- **Zero Prompt & Code Leakage**: O compressor e os hooks nunca capturam, armazenam ou transmitem prompts, código-fonte ou corpos de saída.
- **Zero Path Leakage na Telemetria**: `exec-events.jsonl` e `context-sensor.json` gravam estritamente metadados categorizados (`family`: `pytest` | `vitest` | `go-test` | `git-status`, `exit_class`, contagens brutas de `raw_bytes` e `emitted_bytes`, flag booleana `compacted`). **Caminhos de arquivos (`/Users/.../test_auth.py`), flags CLI, variáveis de ambiente ou argumentos de testes jamais chegam ao disco de telemetria.**
- **Superfície Restrita de Reescrita**: O `execwrap.Decide` só considera comandos sob 512 bytes compostos exclusivamente por caracteres seguros (alfanuméricos, espaços e `- _ . / : = , + @ %`). Proíbe pipes, redirecionamentos, crases, aspas e substituição de comandos.
- **Recursion Guard**: Injeção da variável `DOWNSHIFT_EXEC_DEPTH=1` em processos-filhos para impedir aninhamento acidental ou loops de reescrita.
- **Isolamento de Processos**: Processos filhos rodam em seu próprio process group com encaminhamento transparente de sinais (`SIGTERM`, `SIGINT`, `SIGHUP`, `SIGQUIT`).

### QA (Engenharia de Testes e Confiabilidade)
- **Corpus Adversarial e Fuzzing**: Conjunto abrangente de fixtures cobrindo variações comuns de `pytest` (ex: `pytest-xdist`, flags `-v`, `-s`, warnings deprecados, doctests) e `vitest` (ex: ANSI colors, barras de progresso, multi-threads, testes assíncronos rejeitados).
- **Garantia Inviolável de Fail-Open**: Qualquer suite contendo `FAILED`, `ERROR`, `AssertionError`, `UnhandledPromiseRejection`, traceback ou código de saída diferente de zero é emitida 100% bruta (verbatim). Testes de regressão garantem que um teste quebrado nunca seja omitido ou sintetizado incorretamente.
- **Contrato de Saída Idêntica sob Erro**: Se o compressor estourar limite de memória (`2 MiB`), encontrar caracteres de controle inválidos ou receber bytes no stderr antes do término, o gate imediatamente desiste e faz streaming raw byte a byte.

### Fullstack (Experiência do Desenvolvedor e Integração)
- **Zero-Dependency Runtime**: Executado inteiramente no binário compilado em Go, sem exigir bibliotecas Python (`pytest-json-report`) ou pacotes npm (`vitest-custom-reporter`).
- **Transparência de Compactação**: Adição padronizada do marcador `[downshift exec: compacted N to M bytes]` ao final da saída, permitindo ao desenvolvedor e ao LLM saberem exatamente que a visualização foi resumida de forma segura.
- **Kill-Switch Imediato**: Suporte a desligamento sem reinicialização de daemon via flag `--no-compact` ou variável de ambiente `DOWNSHIFT_EXEC=0`.

