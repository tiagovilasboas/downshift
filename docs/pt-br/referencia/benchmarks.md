# Referência Técnica de Benchmarks de Compactação

> Downshift por Tiago Vilas Boas  
> Repositório oficial: https://github.com/tiagovilasboas/downshift-ai  
> Licença: Apache 2.0

Este documento apresenta a análise técnica detalhada e os resultados oficiais do benchmark de compactação de saídas de comando do **Downshift**, gerados diretamente a partir do corpus sintético de 107 fixtures (`internal/compressor/testdata/corpus`) através do comando `make bench`.

---

## 1. Metodologia do Benchmark

### Critérios de Medição:
- **Corpus Oficial:** Composto por **107 fixtures** reais e sintéticas que simulam transcripts de comandos de desenvolvimento comuns (testes, builds, linters e git).
- **Modo Seguro e Exit 0:** Apenas fixtures que o Downshift reconhece e compacta com sucesso são computadas. Fixtures em formatos não suportados passam diretamente pelo comportamento *fail-open* e não são computadas como taxas falsas de economia.
- **Aproximação de Tokens:** A quantidade de tokens é estimada usando a convenção padrão de engenharia de `bytes / 4` (representando uma média realista para saídas textuais em inglês e código-fonte nos tokenizers modernos).
- **Inclusão do Marcador de Execução:** Tanto os bytes do Downshift quanto os do CCR já incluem o tamanho do marcador informativo injetado ao final de cada execução (`[downshift compact: ...]`).
- **Dobra de Repetições no CCR (*FoldRepeats*):** O modo `with CCR` aplica a dobra de repetições (onde sequências de 8 ou mais linhas repetitivas ou idênticas são resumidas em 3 linhas iniciais, 2 finais e um marcador central). Isso é feito com segurança estrita porque a saída bruta original está salva no disco local para recuperação via `downshift retrieve`.

---

## 2. Resumo Consolidado de Eficiência

| Métrica Avaliada | Tamanho Bruto (Raw) | Downshift (sem CCR) | Economia sem CCR | Downshift (com CCR) | Economia com CCR |
|---|--:|--:|--:|--:|--:|
| **Total do Corpus (107 fixtures)** | **130.570 bytes**<br>*(~32.642 tokens)* | **63.861 bytes**<br>*(~15.965 tokens)* | **51.1%** | **36.559 bytes**<br>*(~9.140 tokens)* | **72.0%** |

### Divisão entre Famílias Sem Perda (*Lossless*) e Com Sumarização (*Lossy*)

O Downshift divide formalmente suas gramáticas em duas categorias de segurança:

1. **Lossless (Sem Perda):** Utilizada em test runners e verificações de integridade. Mantém **100% dos relatórios de erro, mensagens de falha, diffs e stack traces** do código da aplicação, suprimindo apenas linhas redundantes de testes que passaram com sucesso.
2. **Lossy (Com Sumarização Controlada):** Utilizada em compilações, logs de progresso e saídas estruturadas onde o log detalhado é preservado no buffer do CCR para recuperação sob demanda.

| Categoria | Fixtures | Bytes Brutos (Raw) | Bytes Downshift | Economia (%) |
|---|--:|--:|--:|--:|
| **Lossless (Sem Perda)** | **59** | **42.611** | **14.820** | **65.2%** |
| **Lossy (Sumarizada)** | **48** | **87.959** | **49.041** | **44.2%** |

> **Destaque:** Nos test runners da categoria *lossless*, o Downshift obtém **65.2% de economia média de tokens** sem sacrificar absolutamente nenhuma linha de diagnóstico ou stack trace relevante.

---

## 3. Tabela Completa por Família de Comandos

A tabela a seguir reflete a execução oficial do benchmark:

| Família do Comando | Categoria | Fixtures | Bytes Brutos | Bytes Downshift | Redução (%) | Tokens Brutos~ | Tokens Downshift~ | Bytes com CCR | Redução com CCR (%) |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|
| **bazel** | lossy | 2 | 911 | 322 | 64.7% | 227 | 80 | 414 | 54.6% |
| **cargo_build** | lossy | 1 | 440 | 174 | 60.5% | 110 | 43 | 220 | 50.0% |
| **cargo_test** | lossless | 3 | 3.660 | 257 | **93.0%** | 915 | 64 | 395 | 89.2% |
| **cypress** | lossless | 1 | 910 | 153 | **83.2%** | 227 | 38 | 199 | 78.1% |
| **docker_build** | lossy | 1 | 388 | 187 | 51.8% | 97 | 46 | 233 | 39.9% |
| **docker_images** | lossy | 1 | 248 | 124 | 50.0% | 62 | 31 | 170 | 31.5% |
| **docker_ps** | lossy | 1 | 475 | 228 | 52.0% | 118 | 57 | 274 | 42.3% |
| **dotnet_build** | lossy | 1 | 468 | 351 | 25.0% | 117 | 87 | 397 | 15.2% |
| **dotnet_test** | lossless | 1 | 333 | 102 | 69.4% | 83 | 25 | 148 | 55.6% |
| **eslint** | lossy | 3 | 2.091 | 1.053 | 49.6% | 522 | 263 | 1.191 | 43.0% |
| **file_read** | lossy | 1 | 11.594 | 2.413 | 79.2% | 2.898 | 603 | 2.459 | 78.8% |
| **flutter_test** | lossless | 5 | 6.624 | 513 | **92.3%** | 1.656 | 128 | 743 | 88.8% |
| **generic** | lossy | 4 | 41.608 | 33.970 | 18.4% | 10.402 | 8.492 | 2.865 | **93.1%** |
| **gh_checks** | lossless | 1 | 497 | 118 | 76.3% | 124 | 29 | 164 | 67.0% |
| **gh_pr** | lossless | 2 | 1.840 | 1.753 | 4.7% | 460 | 438 | 1.845 | -0.3% |
| **gh_run_list** | lossless | 1 | 378 | 221 | 41.5% | 94 | 55 | 267 | 29.4% |
| **gh_run_log** | lossy | 2 | 2.442 | 637 | 73.9% | 610 | 159 | 729 | 70.1% |
| **git_diff** | lossless | 2 | 541 | 533 | 1.5% | 135 | 133 | 625 | -15.5% |
| **git_log** | lossy | 5 | 2.265 | 610 | 73.1% | 566 | 152 | 840 | 62.9% |
| **git_show** | lossless | 1 | 363 | 367 | -1.1% | 90 | 91 | 413 | -13.8% |
| **git_stash** | lossy | 1 | 192 | 175 | 8.9% | 48 | 43 | 221 | -15.1% |
| **git_status** | lossless | 5 | 1.113 | 538 | 51.7% | 278 | 134 | 768 | 31.0% |
| **glab** | lossy | 2 | 1.673 | 1.672 | 0.1% | 418 | 418 | 1.764 | -5.4% |
| **go_build** | lossy | 2 | 632 | 500 | 20.9% | 158 | 125 | 592 | 6.3% |
| **go_test** | lossless | 13 | 13.395 | 6.044 | 54.9% | 3.348 | 1.511 | 6.642 | 50.4% |
| **golangci_lint** | lossy | 2 | 569 | 397 | 30.2% | 142 | 99 | 489 | 14.1% |
| **gradle** | lossy | 4 | 1.875 | 818 | 56.4% | 468 | 204 | 1.002 | 46.6% |
| **grep_counts** | lossy | 1 | 226 | 114 | 49.6% | 56 | 28 | 160 | 29.2% |
| **grep_rows** | lossless | 1 | 1.920 | 1.298 | 32.4% | 480 | 324 | 998 | 48.0% |
| **http_fetch** | lossy | 1 | 4.980 | 1.841 | 63.0% | 1.245 | 460 | 1.298 | 73.9% |
| **jest** | lossless | 2 | 459 | 208 | 54.7% | 114 | 52 | 300 | 34.6% |
| **kubectl_pods** | lossy | 1 | 229 | 182 | 20.5% | 57 | 45 | 228 | 0.4% |
| **lockfile_diff**| lossy | 1 | 2.479 | 219 | **91.2%** | 619 | 54 | 265 | 89.3% |
| **markdown** | lossy | 2 | 1.063 | 921 | 13.4% | 265 | 230 | 1.013 | 4.7% |
| **maven** | lossy | 3 | 6.676 | 880 | **86.8%** | 1.669 | 220 | 1.018 | 84.8% |
| **mypy** | lossy | 2 | 710 | 373 | 47.5% | 177 | 93 | 465 | 34.5% |
| **next_build** | lossless | 1 | 578 | 135 | 76.6% | 144 | 33 | 181 | 68.7% |
| **npm_audit** | lossy | 1 | 1.052 | 422 | 59.9% | 263 | 105 | 468 | 55.5% |
| **npm_install** | lossless | 1 | 714 | 78 | **89.1%** | 178 | 19 | 124 | 82.6% |
| **nx** | lossless | 1 | 350 | 96 | 72.6% | 87 | 24 | 142 | 59.4% |
| **phpstan** | lossless | 1 | 112 | 70 | 37.5% | 28 | 17 | 116 | -3.6% |
| **phpunit** | lossless | 1 | 260 | 111 | 57.3% | 65 | 27 | 157 | 39.6% |
| **pip_install** | lossless | 1 | 250 | 77 | 69.2% | 62 | 19 | 123 | 50.8% |
| **playwright** | lossless | 2 | 906 | 167 | **81.6%** | 226 | 41 | 259 | 71.4% |
| **progress** | lossy | 1 | 2.181 | 119 | **94.5%** | 545 | 29 | 165 | 92.4% |
| **pytest** | lossless | 2 | 1.216 | 162 | **86.7%** | 304 | 40 | 254 | 79.1% |
| **rspec** | lossless | 1 | 443 | 91 | 79.5% | 110 | 22 | 137 | 69.1% |
| **rubocop** | lossless | 2 | 685 | 492 | 28.2% | 171 | 123 | 584 | 14.7% |
| **ruff** | lossy | 2 | 492 | 339 | 31.1% | 123 | 84 | 431 | 12.4% |
| **tsc** | lossless | 3 | 946 | 204 | 78.4% | 236 | 51 | 342 | 63.8% |
| **turbo** | lossless | 1 | 494 | 101 | 79.6% | 123 | 25 | 147 | 70.2% |
| **vitest** | lossless | 4 | 3.624 | 931 | 74.3% | 906 | 232 | 1.115 | 69.2% |
| **uncompacted** | lossy | 3 | 6.105 | 6.105 | 0.0% | 1.526 | 1.526 | 1.176 | 80.7% |
| **TOTAL** | — | **107** | **130.570** | **63.861** | **51.1%** | **32.642** | **15.965** | **36.559** | **72.0%** |

---

## 4. Análise dos Casos Notáveis

1. **Test Runners com Mais de 80% a 90% de Economia:**
   - `cargo test` (**93.0%**): Corta a lista de crates e dezenas de `test ... ok`, exibindo apenas o resumo de sucesso.
   - `flutter test` (**92.3%**): Condensa streams de eventos assíncronos do Dart/Flutter.
   - `pytest` (**86.7%**): Transforma pontos e linhas repetidas de testes em contadores estruturados.
   - `playwright` (**81.6%**): Remove as linhas de testes que passaram na primeira tentativa mantendo a integridade de testes *flaky* e falhas.
   - `vitest` (**74.3%**): Mantém arquivos com falha intactos a partir do cabeçalho de erro e limpa apenas as árvores de arquivos aprovados.

2. **Diferença do CCR em Saídas Genéricas:**
   - Na família `generic`, a redução sem CCR foi de 18.4%. Com a ativação do CCR e a dobra de repetições (*FoldRepeats*), a economia disparou para **93.1%** (de 41.608 bytes para 2.865 bytes), pois centenas de linhas de logs repetitivos foram dobradas mantendo a garantia de recuperação integral.

3. **Valores Negativos Marginais:**
   - Em saídas minúsculas (como um `git show` ou `git diff` de 300 bytes), a adição do marcador informativo do CCR (`[downshift compact: ...]`) pode adicionar cerca de 40 a 60 bytes. Esse comportamento é esperado e desejável, pois fornece a rastreabilidade necessária para o agente.

---

## 5. A Escada de Evidência (*Evidence Ladder*)

O Downshift adota um compromisso de transparência científica em relação a métricas de economia:

1. **Degrau A — Preço de Tabela Teórico:** Suposições matemáticas de quanto um modelo custaria. O Downshift não baseia seus produtos nisso.
2. **Degrau B — Bytes e Tokens Medidos no Corpus:** O benchmark formal desta página (51.1% e 72.0% de economia medidos byte a byte em 107 fixtures).
3. **Degrau C — Telemetria Local Real:** O que o seu ambiente registrou em `events.jsonl` e `exec-events.jsonl` (consultável via `downshift stats`).
4. **Degrau D — Experimento Controlado A/B com Grupo de Controle:** Medição de tarefas reais comparando execuções com e sem Downshift em condições idênticas.
5. **Degrau E — Fatura e Custo por Tarefa Aceita:** A redução real observada na fatura do provedor de IA no final do mês.

O Downshift nunca inventa valores em dólares ou promete economias milagrosas: cada número reportado pode ser auditado diretamente nos arquivos de log da sua própria máquina.
