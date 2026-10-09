# Native Context Compressor & Context Sensor

Downshift includes an in-process, zero-dependency **Native Context Compressor** (`internal/compressor`) and event-driven **Context Sensor** (`internal/sensor`) to govern token consumption without altering critical diagnostics or introducing external runtimes.

---

## 1. Architectural Principles

1. **Zero External Runtime**: Written purely in Go, with zero Python, Rust, Node, network or LLM dependencies. `context enable` persiste o provider nativo e o contrato de exit code. O PostToolUse do Claude Code grava `CompressExit` em modo observe, com o exit code real, só quando essa flag está ligada. O hook não substitui a saída da ferramenta e não comprime prompt.
2. **Deterministic & Fail-Open**: Compacta sucesso repetido só com exit code 0. **Preserva erro, stack trace, exit diferente de zero, linha que não seja `ok`, busca, lista de arquivos e `git log`.**
3. **Observability vs. Transformability**: Distinguishes observational telemetry (e.g. `PostToolUse` telemetry) from transformational filters (command pipes or tool proxies).
4. **Epistemic Precision**: Metrics clearly distinguish **Observed** counts from **Estimated** heuristics and **Unavailable** harness channels.
5. **Privacy First**: Sensor stores only session hashes, format classifications, and byte aggregates. **Prompts, private code, and sensitive outputs are never persisted.**

---

## 2. Supported Formats & Compression Rules

| Format | Detection Heuristic | Safe Action | Fail-Open Guarantee |
|---|---|---|---|
| `go test` | `ok` package lines | Summarizes a suite only when every non-empty line is `ok` | Any other line, `FAIL`, or non-zero exit is preserved verbatim |
| `git status` | `On branch`, `nothing to commit` | Compacts a clean status | Preserves branch, conflicts and uncommitted changes |
| `git log` | `commit [0-9a-f]{40}` | Preserved verbatim | A summary would drop commit bodies |
| `search` (grep/rg) | `path:line: content` | Preserved verbatim | A prefix would hide matches |
| `file_listing` | `find`, `ls -R`, tree structures | Preserved verbatim | A prefix would hide names |
| `logs` | Repeating log line prefixes | Collapses consecutive identical lines (`[Repeated N times]`) | Preserves first and last timestamp + all error logs |

---

## 3. Economia de tokens

Não há medição reproduzível de economia de tokens neste repositório. Não cite 68, 35 ou 80 como evidência. Tabelas anteriores com 60–98% e com ~80% combinado eram constantes ilustrativas. Elas foram retiradas. `downshift context benchmark` mede bytes em fixtures versionadas (original, reduzido, economia em bytes). Isso não é token e não é dólar. `downshift context metrics` pode contar comandos no log de compactação; os campos de token ficam `unavailable` e não são uma economia medida.

Releitura de histórico aumenta o contexto quando a saída de uma ferramenta volta no turno seguinte. Isso descreve o mecanismo. Não é um número de tokens evitados.

---

## 4. Matriz de roteamento e tabela de bytes

`downshift context benchmark` lista os quatro cenários. Custo de roteamento e acurácia saem como `not measured`. Em seguida o comando roda o compressor em fixtures versionadas (`go test` só com `ok`, `go test` com linha que não é `ok`, logs repetidos, hits de busca) e imprime original, reduzido e economia, com unidade bytes. Não inventa tokens nem dólares. Não cite 68, 35 ou 80.

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
| **Claude Code** | `PostToolUse` (duração, ferramenta, exit code & bytes) | Pipe explícito ou instrução | Com a flag ligada, o hook grava `CompressExit` em observe; não substitui a saída que o modelo vê |
| **Cursor** | Telemetria de logs | Instruções do agente | Compactação determinística guiada |
| **Codex** | Payloads de hook | Execução direta | Filtro Go nativo em processo |
| **Antigravity** | Ciclo de sessão | Filtro direto | Motor nativo zero-dependência |
| **KiroCrew** | Policy mode & telemetria | Instruções / pipes de subagentes | Poda de contexto no respawn (`include_memory=false`) e compactação determinística |
