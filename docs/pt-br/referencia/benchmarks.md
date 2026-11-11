# Referência Técnica de Benchmarks de Compactação

> Downshift por Tiago Vilas Boas  
> Repositório oficial: https://github.com/tiagovilasboas/downshift-ai  
> Licença: Apache 2.0

Este documento apresenta a análise técnica detalhada e os resultados oficiais do benchmark de compactação de saídas de comando do **Downshift**, gerados diretamente a partir do corpus sintético de 113 fixtures compactadas (`internal/compressor/testdata/corpus`) através do comando `make bench`.

---

## 1. Metodologia do Benchmark

### Critérios de Medição:
- **Corpus Oficial:** Composto por **113 fixtures** compactadas, reais e sintéticas, que simulam transcripts de comandos de desenvolvimento comuns (testes, builds, linters e git) e leituras do desktop pelo `agent-desktop`.
- **Modo Seguro e Exit 0:** Apenas fixtures que o Downshift reconhece e compacta com sucesso são computadas. Fixtures em formatos não suportados passam diretamente pelo comportamento *fail-open* e não são computadas como taxas falsas de economia.
- **Aproximação de Tokens:** A quantidade de tokens é estimada usando a convenção padrão de engenharia de `bytes / 4` (representando uma média realista para saídas textuais em inglês e código-fonte nos tokenizers modernos).
- **Inclusão do Marcador de Execução:** Tanto os bytes do Downshift quanto os do CCR já incluem o tamanho do marcador informativo injetado ao final de cada execução (`[downshift compact: ...]`).
- **Dobra de Repetições no CCR (*FoldRepeats*):** O modo `with CCR` aplica a dobra de repetições (onde sequências de 8 ou mais linhas repetitivas ou idênticas são resumidas em 3 linhas iniciais, 2 finais e um marcador central). Isso é feito com segurança estrita porque a saída bruta original está salva no disco local para recuperação via `downshift retrieve`.

---

## 2. Resumo Consolidado de Eficiência

| Métrica Avaliada | Tamanho Bruto (Raw) | Downshift (sem CCR) | Economia sem CCR | Downshift (com CCR) | Economia com CCR |
|---|--:|--:|--:|--:|--:|
| **Total do Corpus (113 fixtures)** | **194.873 bytes**<br>*(~48.718 tokens)* | **92.359 bytes**<br>*(~23.089 tokens)* | **52.6%** | **59.774 bytes**<br>*(~14.943 tokens)* | **69.3%** |

### Divisão entre Famílias Sem Perda (*Lossless*) e Com Sumarização (*Lossy*)

O Downshift divide formalmente suas gramáticas em duas categorias de segurança:

1. **Lossless (Sem Perda):** Utilizada em test runners e verificações de integridade. Mantém **100% dos relatórios de erro, mensagens de falha, diffs e stack traces** do código da aplicação, suprimindo apenas linhas redundantes de testes que passaram com sucesso.
2. **Lossy (Com Sumarização Controlada):** Utilizada em compilações, logs de progresso e saídas estruturadas onde o log detalhado é preservado no buffer do CCR para recuperação sob demanda.

| Categoria | Fixtures | Bytes Brutos (Raw) | Bytes Downshift | Economia (%) |
|---|--:|--:|--:|--:|
| **Lossless (Sem Perda)** | **64** | **104.386** | **42.439** | **59.3%** |
| **Lossy (Sumarizada)** | **49** | **90.487** | **49.920** | **44.8%** |

> **Destaque:** Nas famílias *lossless*, o Downshift obtém **59.3% de economia média de bytes** sem sacrificar absolutamente nenhuma linha de diagnóstico ou stack trace relevante.

---

## 3. Tabela Completa por Família de Comandos

A tabela a seguir reflete a execução oficial do benchmark:

| Família do Comando | Categoria | Fixtures | Bytes Brutos | Bytes Downshift | Redução (%) | Tokens Brutos~ | Tokens Downshift~ | Bytes com CCR | Redução com CCR (%) |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|
| **agent_desktop** | lossless | 3 | 61.032 | 26.329 | 56.9% | 15.258 | 6.582 | 26.371 | 56.8% |
| **bazel** | lossy | 2 | 911 | 340 | 62.7% | 227 | 85 | 368 | 59.6% |
| **cargo_build** | lossy | 1 | 440 | 183 | 58.4% | 110 | 45 | 197 | 55.2% |
| **cargo_test** | lossless | 3 | 3.660 | 284 | **92.2%** | 915 | 71 | 299 | 91.8% |
| **cypress** | lossless | 1 | 910 | 162 | **82.2%** | 227 | 40 | 176 | 80.7% |
| **docker_build** | lossy | 1 | 388 | 196 | 49.5% | 97 | 49 | 210 | 45.9% |
| **docker_images** | lossy | 1 | 248 | 133 | 46.4% | 62 | 33 | 147 | 40.7% |
| **docker_ps** | lossy | 1 | 475 | 237 | 50.1% | 118 | 59 | 251 | 47.2% |
| **dotnet_build** | lossy | 1 | 468 | 360 | 23.1% | 117 | 90 | 374 | 20.1% |
| **dotnet_test** | lossless | 1 | 333 | 111 | 66.7% | 83 | 27 | 125 | 62.5% |
| **eslint** | lossy | 3 | 2.091 | 1.080 | 48.4% | 522 | 270 | 1.122 | 46.3% |
| **file_read** | lossy | 1 | 11.594 | 2.422 | 79.1% | 2.898 | 605 | 2.436 | 79.0% |
| **flutter_test** | lossless | 5 | 6.624 | 558 | **91.6%** | 1.656 | 139 | 583 | 91.2% |
| **generic** | lossy | 4 | 41.608 | 34.006 | 18.3% | 10.402 | 8.501 | 2.773 | 93.3% |
| **gh_checks** | lossless | 1 | 515 | 127 | 75.3% | 128 | 31 | 141 | 72.6% |
| **gh_pr** | lossless | 2 | 1.843 | 1.774 | 3.7% | 460 | 443 | 1.802 | 2.2% |
| **gh_run_list** | lossless | 1 | 378 | 230 | 39.2% | 94 | 57 | 244 | 35.4% |
| **gh_run_log** | lossy | 2 | 2.451 | 655 | 73.3% | 612 | 163 | 683 | 72.1% |
| **git_diff** | lossless | 3 | 810 | 849 | -4.8% | 202 | 212 | 891 | -10.0% |
| **git_log** | lossy | 5 | 2.265 | 655 | 71.1% | 566 | 163 | 725 | 68.0% |
| **git_show** | lossless | 2 | 786 | 828 | -5.3% | 196 | 207 | 856 | -8.9% |
| **git_stash** | lossy | 1 | 192 | 184 | 4.2% | 48 | 46 | 198 | -3.1% |
| **git_status** | lossless | 5 | 1.113 | 583 | 47.6% | 278 | 145 | 653 | 41.3% |
| **glab** | lossy | 2 | 1.673 | 1.690 | -1.0% | 418 | 422 | 1.718 | -2.7% |
| **go_build** | lossy | 2 | 632 | 518 | 18.0% | 158 | 129 | 546 | 13.6% |
| **go_test** | lossless | 13 | 13.425 | 6.167 | 54.1% | 3.356 | 1.541 | 4.546 | 66.1% |
| **golangci_lint** | lossy | 2 | 569 | 415 | 27.1% | 142 | 103 | 443 | 22.1% |
| **gradle** | lossy | 4 | 1.875 | 854 | 54.5% | 468 | 213 | 910 | 51.5% |
| **grep_counts** | lossy | 1 | 226 | 123 | 45.6% | 56 | 30 | 137 | 39.4% |
| **grep_rows** | lossless | 1 | 1.920 | 1.307 | 31.9% | 480 | 326 | 975 | 49.2% |
| **http_fetch** | lossy | 1 | 4.980 | 1.850 | 62.9% | 1.245 | 462 | 1.275 | 74.4% |
| **jest** | lossless | 2 | 459 | 226 | 50.8% | 114 | 56 | 254 | 44.7% |
| **kubectl_pods** | lossy | 1 | 229 | 191 | 16.6% | 57 | 47 | 205 | 10.5% |
| **lockfile_diff** | lossy | 2 | 4.998 | 675 | **86.5%** | 1.249 | 168 | 703 | 85.9% |
| **markdown** | lossy | 2 | 1.063 | 939 | 11.7% | 265 | 234 | 967 | 9.0% |
| **maven** | lossy | 3 | 6.676 | 907 | **86.4%** | 1.669 | 226 | 949 | 85.8% |
| **mypy** | lossy | 2 | 710 | 391 | 44.9% | 177 | 97 | 419 | 41.0% |
| **next_build** | lossless | 1 | 578 | 144 | 75.1% | 144 | 36 | 158 | 72.7% |
| **npm_audit** | lossy | 1 | 1.052 | 431 | 59.0% | 263 | 107 | 445 | 57.7% |
| **npm_install** | lossless | 1 | 714 | 87 | **87.8%** | 178 | 21 | 101 | 85.9% |
| **nx** | lossless | 1 | 350 | 105 | 70.0% | 87 | 26 | 119 | 66.0% |
| **phpstan** | lossless | 1 | 112 | 79 | 29.5% | 28 | 19 | 93 | 17.0% |
| **phpunit** | lossless | 1 | 260 | 120 | 53.8% | 65 | 30 | 134 | 48.5% |
| **pip_install** | lossless | 1 | 250 | 86 | 65.6% | 62 | 21 | 100 | 60.0% |
| **playwright** | lossless | 2 | 906 | 185 | 79.6% | 226 | 46 | 197 | 78.3% |
| **progress** | lossy | 1 | 2.181 | 128 | **94.1%** | 545 | 32 | 142 | 93.5% |
| **pytest** | lossless | 2 | 1.216 | 180 | **85.2%** | 304 | 45 | 190 | 84.4% |
| **rspec** | lossless | 1 | 443 | 100 | 77.4% | 110 | 25 | 114 | 74.3% |
| **rubocop** | lossless | 2 | 685 | 510 | 25.5% | 171 | 127 | 538 | 21.5% |
| **ruff** | lossy | 2 | 492 | 357 | 27.4% | 123 | 89 | 385 | 21.7% |
| **tsc** | lossless | 3 | 946 | 231 | 75.6% | 236 | 57 | 273 | 71.1% |
| **turbo** | lossless | 1 | 494 | 110 | 77.7% | 123 | 27 | 124 | 74.9% |
| **vitest** | lossless | 4 | 3.624 | 967 | 73.3% | 906 | 241 | 989 | 72.7% |
| **uncompacted** | lossy | 3 | 6.150 | 6.150 | 0.0% | 1.537 | 1.537 | 1.152 | 81.3% |
| **TOTAL** | — | **113** | **194.873** | **92.359** | **52.6%** | **48.718** | **23.089** | **59.774** | **69.3%** |

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
   - Em saídas minúsculas (como um `git show` ou `git diff` de 300 bytes), a adição do marcador informativo do CCR (`[downshift compact: ...]`) pode adicionar algumas dezenas de bytes. Esse comportamento é esperado e desejável, pois fornece a rastreabilidade necessária para o agente.

---

## 5. A Escada de Evidência (*Evidence Ladder*)

O Downshift adota um compromisso de transparência científica em relação a métricas de economia:

1. **Degrau A — Preço de Tabela Teórico:** Suposições matemáticas de quanto um modelo custaria. O Downshift não baseia seus produtos nisso.
2. **Degrau B — Bytes e Tokens Medidos no Corpus:** O benchmark formal desta página (52.6% e 69.3% de economia medidos byte a byte em 113 fixtures).
3. **Degrau C — Telemetria Local Real:** O que o seu ambiente registrou em `events.jsonl` e `exec-events.jsonl` (consultável via `downshift stats`).
4. **Degrau D — Experimento Controlado A/B com Grupo de Controle:** Medição de tarefas reais comparando execuções com e sem Downshift em condições idênticas.
5. **Degrau E — Fatura e Custo por Tarefa Aceita:** A redução real observada na fatura do provedor de IA no final do mês.

O Downshift nunca inventa valores em dólares ou promete economias milagrosas: cada número reportado pode ser auditado diretamente nos arquivos de log da sua própria máquina.
