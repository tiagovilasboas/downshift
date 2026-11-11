# Guia de Início Rápido — Downshift

> Downshift por Tiago Vilas Boas  
> Repositório oficial: https://github.com/tiagovilasboas/downshift-ai  
> Licença: Apache 2.0

Configure o **Downshift** no seu ambiente de desenvolvimento em menos de 5 minutos e comece a economizar tokens de contexto imediatamente.

---

## Pré-requisitos

- **Sistema Operacional:** macOS, Linux ou Windows (via WSL ou nativo)
- **Harness de IA:** Pelo menos um agente suportado instalado no ambiente (Claude Code, Codex, Cursor, OpenCode, Antigravity ou KiroCrew)
- **Terminal:** `bash`, `zsh` ou shell equivalente

---

## Passo 1: Instalação do Binário

Execute o instalador oficial via curl no seu terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift-ai/main/install.sh | sh
```

O script baixa o binário pré-compilado para a sua arquitetura e sistema operacional e o posiciona no seu `PATH` (geralmente em `/usr/local/bin` ou `~/.local/bin`).

### Alternativas de Instalação

- **Via Go Toolchain (se possuir Go 1.27+ instalado):**
  ```bash
  go install github.com/tiagovilasboas/downshift-ai/cmd/downshift@latest
  ```
- **Binários Manuais:**
  Acesse a página de [Releases no GitHub](https://github.com/tiagovilasboas/downshift-ai/releases), baixe o arquivo comprimido compatível e confira o hash com o arquivo `checksums.txt`.

Para confirmar a instalação bem-sucedida:

```bash
downshift version
```

---

## Passo 2: Configuração dos Harnesses (`downshift setup`)

O Downshift acompanha um assistente inteligente e seguro para configurar a integração com os seus agentes instalados:

```bash
downshift setup
```

### O que o `setup` faz:
1. **Varredura de Harnesses:** Detecta automaticamente instalações ativas do Claude Code, Codex, Cursor, OpenCode, Antigravity e KiroCrew.
2. **Exibição Prévia de Diff:** Mostra exatamente o que será alterado ou adicionado nos arquivos de configuração do seu usuário (por exemplo, ganchos de `PreToolUse`, registro do servidor MCP e skills de orquestração).
3. **Confirmação Explícita:** Nada é gravado em disco sem o seu consentimento.
4. **Criação de Backup:** Antes de qualquer alteração, o Downshift cria um backup dos arquivos originais permitindo restauração imediata com `downshift setup --undo`.

Se preferir simular a execução sem alterar nada, utilize:

```bash
downshift setup --dry-run
```

---

## Passo 3: Verificação com `downshift doctor`

Após executar o setup, valide se todas as peças estão funcionando em perfeita harmonia:

```bash
downshift doctor
```

O `doctor` realiza um diagnóstico detalhado:
- ✅ **Binário:** Verifica se o executável `downshift` está acessível no `PATH`.
- ✅ **Hooks:** Inspeciona se os ganchos interceptores estão instalados e configurados corretamente para cada harness ativo.
- ✅ **Diretório de Estado:** Valida permissões de leitura e escrita em `~/.downshift/`.
- ✅ **Servidor MCP:** Testa a prontidão das 13 ferramentas nativas expostas pelo Downshift.

Se alguma checagem falhar, o `doctor` exibe uma linha informativa com a sugestão exata do comando para corrigir a pendência.

---

## Passo 4: Executando o Primeiro Comando Compactado

A compactação de comandos via compactor funciona como um comando de terminal comum e pode ser testada imediatamente:

```bash
downshift compact -- go test ./...
```

Ou com o Git:

```bash
downshift compact -- git status
```

### O que acontece na prática?

Imagine uma suíte de testes com dezenas de pacotes que produzia mais de 3.900 bytes de saída para apenas dizer que tudo passou. O Downshift condensa essa saída para menos de 1.000 bytes:

```text
Go test: PASS (53 packages ok, 4 without tests) under github.com/tiagovilasboas/downshift-ai/internal/
ok accounting
ok adapters/antigravity
...
[downshift compact: 3956 to 998 bytes; downshift retrieve 4a12b8c9d0e1]
```

- **Economia Imediata:** Mais de 70% de redução de tokens gastos desnecessariamente na janela de contexto.
- **Transparência de Status:** O status de saída (`PASS` ou `FAIL`) e o código de saída (*exit code*) do processo são transmitidos de forma fidedigna.

---

## Passo 5: Recuperando a Saída Bruta via CCR

Note a linha final inserida pelo compactor com o identificador do CCR:
`[downshift compact: ... downshift retrieve 4a12b8c9d0e1]`

O CCR (*Command Context Recovery*) armazena localmente a saída original completa. Se o seu agente ou você precisar ver qualquer linha suprimida:

```bash
downshift retrieve 4a12b8c9d0e1
```

Você também pode filtrar por padrões de erro ou intervalos de linhas:

```bash
downshift retrieve 4a12b8c9d0e1 --grep "FAIL|panic"
downshift retrieve 4a12b8c9d0e1 --lines 1:50
```

---

## Passo 6: Monitorando o Roteamento de Subagentes

Quando você utilizar seu agente habitual (por exemplo, no Claude Code ou Codex) e ele disparar um subagente, o Downshift interceptará a chamada e aplicará o modelo ideal para a complexidade da demanda.

Para inspecionar em tempo real o que o Downshift decidiu na sua sessão atual:

```bash
# Visualizar a árvore de subagentes da sessão atual
downshift session

# Entender em linguagem natural o motivo de cada decisão tomada
downshift session --why
```

Para ver as estatísticas consolidadas da máquina:

```bash
downshift stats
```

---

## Passo 7: Invocando o Squad com Gatilhos de Contexto

No chat do seu agente, experimente descrever um objetivo e ativar o time completo de especialistas com o gatilho de contexto:

```text
Usuário: Precisamos implementar autenticação OAuth2 com testes de integração e análise de segurança
Usuário: downshift full-power
```

Ou utilize o gatilho canônico em português:

```text
Usuário: potencia-total
```

O orquestrador do Downshift analisará as cotas disponíveis, dividirá o trabalho em fatias verticais isoladas e delegará para até 5 agentes simultâneos (`coder`, `tester`, `security`, etc.), cada um operando no tier de modelo mais econômico e eficiente.

---

## Próximos Passos

Agora que seu ambiente está configurado e validado, aprofunde-se nos recursos avançados:

- 📦 [Guia de Compactação e CCR](compactacao-e-ccr.md) — Saiba como customizar regras de compactação e retenção de logs.
- 🔀 [Guia de Roteamento de Modelos](roteamento-de-modelos.md) — Aprenda a criar regras personalizadas em `policy.json`.
- 📊 [Referência de Benchmarks](../referencia/benchmarks.md) — Veja os dados oficiais de performance e economia em 113 fixtures.
- 📖 [Hub Principal PT-BR](../README.md) — Voltar à página principal de documentação.
