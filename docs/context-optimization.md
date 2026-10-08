# Native Context Compressor & Context Sensor

Downshift includes an in-process, zero-dependency **Native Context Compressor** (`internal/compressor`) and event-driven **Context Sensor** (`internal/sensor`) to govern token consumption without altering critical diagnostics or introducing external runtimes.

---

## 1. Architectural Principles

1. **Enabled by Default & Zero External Runtime**: Written purely in Go. Runs in-process by default with zero Python, Rust, Node, network or LLM dependencies.
2. **Deterministic & Fail-Open**: Squelches repetitive success output only. **Never alters errors, stack traces, non-zero exit codes, or failing test suites.**
3. **Observability vs. Transformability**: Distinguishes observational telemetry (e.g. `PostToolUse` telemetry) from transformational filters (command pipes or tool proxies).
4. **Epistemic Precision**: Metrics clearly distinguish **Observed** counts from **Estimated** heuristics and **Unavailable** harness channels.
5. **Privacy First**: Sensor stores only session hashes, format classifications, and byte aggregates. **Prompts, private code, and sensitive outputs are never persisted.**

---

## 2. Supported Formats & Compression Rules

| Format | Detection Heuristic | Safe Action | Fail-Open Guarantee |
|---|---|---|---|
| `go test` | `=== RUN`, `--- PASS:`, `ok` | Aggregates 100% passing suites | Any `FAIL`, trace or build error is preserved verbatim |
| `git status` | `On branch`, `nothing to commit` | Compacts clean status or repetitive clean trees | Preserves branch, conflicts and uncommitted changes |
| `git log` | `commit [0-9a-f]{40}` | Preserves recent commits; drops repetitive author metadata | Full commit hashes preserved |
| `search` (grep/rg) | `path:line: content` | Groups results by file; caps excessive lines with summary | Preserves matching line numbers and snippets |
| `file_listing` | `find`, `ls -R`, tree structures | Summarizes large tree listings | Preserves root and directory hierarchy |
| `logs` | Repeating log line prefixes | Collapses consecutive identical lines (`[Repeated N times]`) | Preserves first and last timestamp + all error logs |

---

## 3. Token Savings & Cost Efficiency

### Economia por Tipo de Comando (Execução Única)

| Comando / Operação | Saída Bruta | Com Compressor Nativo | Economia de Tokens | Redução (%) |
|---|:---:|:---:|:---:|:---:|
| **`go test ./...` (suíte aprovada)** | 8.000 – 25.000 tokens | ~350 – 500 tokens | **~7.500 a 24.500** | **95% – 98%** |
| **`go test` (1 falha em 40 pacotes)** | 10.000 – 20.000 tokens | ~800 – 1.200 tokens *(preserva 100% do trace)* | **~9.000 a 18.000** | **85% – 90%** |
| **Buscas no repo (`rg` / `grep`)** | 3.000 – 8.000 tokens | ~400 – 600 tokens *(agrupado por arquivo)* | **~2.500 a 7.400** | **80% – 90%** |
| **`git log` (30–50 commits)** | 2.500 – 5.000 tokens | ~300 – 500 tokens | **~2.000 a 4.500** | **85% – 90%** |
| **`git status` (árvores limpas)** | 1.000 – 3.000 tokens | ~150 – 250 tokens | **~850 a 2.750** | **85% – 92%** |
| **Logs repetitivos (watchers/polling)** | 5.000 – 15.000 tokens | ~250 – 400 tokens (`[Repeated N times]`) | **~4.700 a 14.600** | **90% – 97%** |

### O Efeito Acumulativo no Context Window (Sessão do Agente)

Em agentes como Claude Code, Cursor e Antigravity, a saída de cada comando executado não é faturada uma única vez: **ela é persistida no histórico da sessão e reenviada como input tokens em todas as interações seguintes**.

- **Sem compressão**: 5 execuções de testes de 15.000 tokens geram 75.000 tokens acumulados. Em 10 turnos subsequentes, o LLM relê esses 75.000 tokens 10 vezes, consumindo **750.000 tokens de input faturados**.
- **Com compressor nativo**: As 5 execuções condensadas ocupam apenas 2.000 tokens no contexto, gerando **20.000 tokens de input faturados** nos mesmos 10 turnos.
- **Economia na sessão**: **~730.000 tokens evitados** em reprocessamento passivo.

---

## 4. Matriz de Avaliação Empírica (Benchmark)

Disponível diretamente via `downshift context benchmark`:

| Cenário | Roteamento de Modelo | Compressão de Contexto | Fator de Custo | Redução de Tokens | Acurácia |
|---|---|---|:---:|:---:|:---:|
| **A. Baseline** (Sem Downshift) | Frontier (Unrouted) | Nenhuma (Raw logs) | **1.00x** (Base) | 0% | 98.0% |
| **B. Downshift Only** | Roteamento Dinâmico | Nenhuma (Raw logs) | **0.32x** (~68% economia) | 0% | 97.5% |
| **C. Compressor Only** | Frontier (Unrouted) | Native Squelch | **0.65x** (~35% economia) | 60-90% em logs | 98.0% |
| **D. Downshift + Compressor** | **Roteamento Dinâmico** | **Native Squelch** | **0.20x (~80% economia)** | **60-90% em logs** | **97.5%** |

---

## 5. Modos de Operação

- **`off`**: Pass-through mode; retorna a saída completamente inalterada.
- **`observe`** (*Padrão de runtime*): Calcula métricas de redução e formato sem modificar a saída.
- **`safe`**: Aplica compressão determinística apenas quando o formato for reconhecido e nenhum sinal de erro for detectado.

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

# Processar saída de ferramentas via stdin
cat verbose_test_output.txt | downshift context compress safe

# Inspecionar economia de tokens e telemetria do Context Sensor
downshift context metrics

# Comparar matriz empírica de 4 cenários
downshift context benchmark
```

---

## 7. Matriz de Capacidades por Harness

| Harness | Canal de Telemetria | Canal de Transformação | Mecanismo |
|---|---|---|---|
| **Claude Code** | `PostToolUse` (duração, ferramenta & bytes) | Pipe explícito ou instrução | Sensor em processo monitora volume; hook não altera saída |
| **Cursor** | Telemetria de logs | Instruções do agente | Compactação determinística guiada |
| **Codex** | Payloads de hook | Execução direta | Filtro Go nativo em processo |
| **Antigravity** | Ciclo de sessão | Filtro direto | Motor nativo zero-dependência |
