# Guia de Compactação e CCR (Command Context Recovery)

> Downshift por Tiago Vilas Boas  
> Repositório oficial: https://github.com/tiagovilasboas/downshift-ai  
> Licença: Apache 2.0

O **Compactor** do Downshift reduz a saída verborrágica de comandos de terminal, preservando rigorosamente os códigos de saída (*exit codes*) e cada detalhe das falhas, enquanto o **CCR** (*Command Context Recovery*) garante que nenhuma informação seja perdida permanentemente ao manter a saída bruta acessível localmente.

---

## 1. O Problema da Verborragia no Contexto

Agentes de desenvolvimento baseados em IA sofrem com o rápido esgotamento de suas janelas de contexto. Comandos triviais de compilação, testes unitários ou inspeção de controle de versão frequentemente produzem milhares de bytes irrelevantes:

- Uma suíte com 50 pacotes em Go que passam com sucesso consome cerca de 2.000 tokens apenas para listar `ok pacote ...`.
- Uma instalação de pacotes via `npm` ou `cargo` gera telas inteiras de progresso e downloads concluídos.
- Um `git status` em um repositório limpo gera múltiplos parágrafos de explicações padrão.

Quando essa massa de texto entra no contexto do modelo, ocorrem três problemas graves:
1. **Desperdício Financeiro:** Tokens consumidos sem gerar valor de engenharia.
2. **Degradação de Atenção (*Context Rot*):** O LLM perde foco em regras de negócio e arquivos prioritários ao tentar processar logs gigantescos.
3. **Truncamento de Histórico:** A conversa é forçada a rodar compactações prematuras de histórico.

---

## 2. Como Funciona o Compactor

O Compactor do Downshift atua como um wrapper transparente em torno de ferramentas permitidas (*allowlisted*):

```bash
downshift compact -- <comando> [argumentos...]
```

*(Nota: `compactor` é o nome oficial do produto e comando; `compact` e `exec` são mantidos como aliases canônicos).*

### Fluxo de Execução Passo a Passo:

1. **Execução sem Shell:** O Downshift executa o binário diretamente via chamada de sistema do sistema operacional (evitando injeções ou efeitos colaterais de subshells).
2. **Captura Fiel de Streams:** O `stdout` e o `stderr` são lidos e temporizados.
3. **Checagem de Status e Erros:** O código de saída do processo é checado. Se o comando teve saída não zero ou escreveu em `stderr`, entram em ação as regras específicas de falha.
4. **Aplicação de Gramática Especializada:** Se a ferramenta for reconhecida (por exemplo, `vitest`, `go test`, `pytest`), sua gramática especializada resume apenas as linhas redundantes de sucesso.
5. **Cadeia Genérica de Compactação:** Se o comando não tiver um compressor específico mas pertencer às famílias de ferramentas conhecidas, ele passa pela esteira genérica de segurança.
6. **Emissão de Marcador:** A saída compactada é impressa no terminal terminando com o identificador exclusivo do CCR para recuperação.

---

## 3. Famílias de Comandos Permitidas

O Compactor suporta dezenas de utilitários populares do ecossistema de software:

### Test Runners
- `go test`
- `vitest` (versões 3, 4 e 5)
- `jest`
- `pytest`
- `cargo test`
- `playwright test` (repórter de lista)
- `cypress run`
- `phpunit` e `pest`
- `flutter test` e `dart test`
- `dotnet test`
- `rspec`

### Controle de Versão e Ferramentas GitHub
- `git status`, `git diff`, `git log`, `git show`, `git stash`, `git branch`, etc.
- `gh pr`, `gh run`, `gh checks`, `gh issue`, `gh repo`, `gh api`

### Compiladores e Linters
- `tsc` (TypeScript Compiler)
- `golangci-lint`, `go vet`, `staticcheck`, `govulncheck`
- `go build`, `cargo build`, `dotnet build`
- `ruff`, `mypy`, `black`, `flake8`
- `eslint`, `prettier --check`, `biome check`
- `rubocop`, `phpstan`

### Ferramentas de Build e Gerenciadores de Pacote
- `npm install`, `npm audit`, `pnpm`, `yarn`, `bun`
- `pip install`, `uv pip`, `poetry`
- `cargo clean`, `cargo check`, `cargo clippy`
- `maven`, `gradle`, `make`, `ninja`, `bazel`
- `turbo run`, `nx run`
- `docker build`, `docker images`, `docker ps`
- `kubectl get/describe/logs`, `helm status`

Qualquer comando fora das famílias permitidas é recusado pelo compactor (código de saída 126), garantindo que comandos arbitrários ou scripts desconhecidos não sejam alterados.

---

## 4. Preservação Rigorosa de Erros e Códigos de Saída

Uma premissa primordial do Downshift é: **a compactação nunca deve esconder problemas ou induzir o agente ao erro**.

### Garantias Absolutas:

1. **Códigos de Saída Idênticos:** Se um comando falha com código 1, 2 ou qualquer outro valor, o Downshift retorna **exatamente o mesmo código de saída**. Se o teste falhou, o teste continua vermelho.
2. **Preservação Byte a Byte de Falhas:** Em test runners que falham (como `go test`, `vitest`, `playwright`, `jest`, `pytest` e `cargo test`), **nenhuma linha de falha é removida**. São preservados intactos:
   - Blocos de `FAIL` e mensagens de erro do compilador
   - Asserções completas, valores esperados (*expected*) vs valores recebidos (*actual*)
   - *Code frames* (trechos de código com indicador de linha do erro)
   - *Stack traces* completos e exceções não tratadas
   - Diffs estruturados de dados
3. **O que é cortado em caso de falha?**  
   Apenas os testes que **passaram com sucesso** na mesma rodada! Em vez de poluir o log com centenas de linhas de testes verdes e obrigar o agente a caçar a falha no meio do scrollback, o Downshift corta as linhas de sucesso e insere um aviso claro no topo:
   `Go test: FAIL (failures kept verbatim; 48 passing package lines cut)`
4. **Comportamento Fail-Open:** Se a saída do teste com falha contiver caracteres estranhos, encoding inválido, streams de erro desconhecidos ou se o formato não puder ser compreendido com 100% de certeza, o Downshift aborta a compactação e entrega a saída bruta completa sem modificação.

---

## 5. Funcionamento do CCR (Command Context Recovery)

O **CCR** (*Command Context Recovery*) é a salvaguarda local que permite ao Downshift compactar agressivamente saídas de terminal com a tranquilidade de que **100% do dado original pode ser recuperado a qualquer instante**.

Desde 6 de novembro de 2026, o CCR vem **ativado por padrão**.

### O Ciclo de Vida do CCR:

1. **Armazenamento Seguro:** A saída original não modificada é salva em `~/.downshift/ccr/<id>.txt`.
2. **Identificador Único:** O `<id>` é um hash criptográfico seguro do conteúdo armazenado (ex: `3f9a1c2b7d40`).
3. **Marcador de Notificação:** O compactor inclui uma linha informativa ao final da saída compactada:
   ```text
   [downshift compact: 12450 to 1120 bytes; downshift retrieve 3f9a1c2b7d40]
   ```
4. **Dobra de Repetições (*FoldRepeats*):** Sequências de 8 ou mais linhas com o mesmo padrão estrutural (como barras de progresso, leituras sucessivas de arquivos ou prints repetidos) são dobradas, preservando as 3 primeiras, as 2 últimas e uma linha informativa central `[... 45 similar lines folded]`. Essa dobra só acontece porque o arquivo bruto está salvo no CCR.

---

## 6. Recuperando Informações com `downshift retrieve`

Tanto o desenvolvedor quanto os agentes de IA podem recuperar o conteúdo completo ou recortes específicos usando o utilitário `retrieve`:

```bash
# Recuperar toda a saída original
downshift retrieve 3f9a1c2b7d40

# Filtrar linhas por expressão regular (ex: buscar pânicos ou falhas)
downshift retrieve 3f9a1c2b7d40 --grep "panic|Error:"

# Visualizar apenas as primeiras ou últimas N linhas
downshift retrieve 3f9a1c2b7d40 --head 30
downshift retrieve 3f9a1c2b7d40 --tail 50

# Extrair um intervalo específico de linhas
downshift retrieve 3f9a1c2b7d40 --lines 120:180
```

### Códigos de Saída do `retrieve`:
- `0`: Sucesso na recuperação.
- `1`: Identificador não encontrado ou expirado.
- `2`: Argumentos inválidos ou sintaxe incorreta.

### Uso via MCP (Model Context Protocol):
O Downshift expõe a ferramenta nativa `retrieve` através de seu servidor MCP local (`downshift mcp serve`). Assim, os próprios agentes de IA (como Claude Code, Cursor ou Antigravity) podem chamar o tool `retrieve(id, grep, head, tail, lines)` autonomamente quando decidirem inspecionar detalhes adicionais.

---

## 7. Políticas de Armazenamento, Segurança e Privacidade

O CCR foi projetado com isolamento e privacidade rigorosos:

- **Permissões do Sistema Operacional:** No Linux e macOS, o diretório `~/.downshift/ccr` é criado com permissão restrita `0700` e os arquivos de saída com modo `0600` (legíveis unicamente pelo usuário proprietário do processo). Em sistemas Windows, herdam as ACLs protegidas do perfil do usuário.
- **Expiração Automática:** Cada arquivo gravado no CCR expira e é excluído automaticamente após **24 horas**.
- **Limites de Tamanho e Expurgo:**
  - Cada execução individual é limitada a no máximo 32 MiB (era 8 MiB até 2026-11-10; subiu para caber a saída acima de 2 MB que o Compactor grava em arquivo temporário e corta em início e fim).
  - O volume total ocupado pela pasta de CCR é monitorado e limitado a 128 MiB; execuções antigas são expurgadas quando o teto é atingido.
- **Sanitização Heurística de Segredos:**
  Antes de salvar a saída no disco, o CCR examina as linhas em busca de padrões sensíveis (chaves privadas, tokens de acesso, cabeçalhos de autorização, cookies, credenciais em URLs e blocos PEM). Linhas que contenham esses padrões são censuradas com:
  `[downshift ccr: line withheld, it looked like a secret]`
- **Sem Links Simbólicos Estranhos:** O gerenciador de armazenamento recusa diretórios que sejam links simbólicos criados por outros usuários, evitando ataques de sobrescrita de arquivos locais.

---

## 8. Configuração e Variáveis de Ambiente

Você pode ajustar o comportamento do CCR conforme a necessidade do seu ambiente:

| Configuração | Descrição |
|---|---|
| Padrão (sem variáveis) | Armazena todas as execuções compactadas (sucesso e falha) |
| `"ccr": false` em `context-optimization.json` | Armazena apenas saídas de comandos que **falharam** |
| `"ccr": false` e `"ccr_failures": false` | Desativa totalmente o armazenamento do CCR |
| `DOWNSHIFT_CCR=0` | Desativa o CCR forçadamente em qualquer situação |
| `DOWNSHIFT_CCR=1` | Força a ativação do CCR para todas as execuções |
| `DOWNSHIFT_CCR_FAILURES=0` ou `1` | Liga/desliga o modo exclusivo para falhas |
| `DOWNSHIFT_EXEC=0` | Desativa a compactação por completo (tudo passa raw) |

As variáveis de ambiente sempre têm precedência sobre os arquivos de configuração locais.

---

## 9. Contabilidade e Telemetria Transparente

Um princípio essencial de engenharia do Downshift é a **honestidade de métricas**:
> Uma economia que o agente desfaz lendo os dados de volta com `retrieve` não é uma economia real.

Toda vez que um comando `downshift retrieve` é chamado, a quantidade de bytes lidos de volta é registrada e **subtraída imediatamente** dos cálculos de economia líquida nos relatórios do `downshift stats` e `downshift report`.

Se você compactou 10.000 bytes para 1.000 bytes (economia de 9.000 bytes), mas o agente executou um `retrieve` de 4.000 bytes, o Downshift reportará uma economia líquida exata de 5.000 bytes.
