
### 📦 Tarefas Compactadas em 2026-10-28 01:09:18 (3 itens)
- [x] F0.1: DX Zero-Prompt & CLI Pura (`downshift` sem argumentos)
- [x] F0.2: Comando `downshift uninstall`
- [x] F0.3: Setup Interativo `--tui`

### 📦 Tarefas Compactadas em 2026-10-28 01:13:55 (4 itens)
- [x] F0.1: DX Zero-Prompt & CLI Pura (`downshift` sem argumentos)
- [x] F0.2: Comando `downshift uninstall`
- [x] F0.3: Setup Interativo `--tui`
- [x] F1.1: Sensor de Falha & Buffer de Dead-Ends (`internal/cascade`)

### 📦 Tarefas Compactadas em 2026-10-28 01:51:01 (3 itens)
- [x] F0.4: Descoberta Dinâmica de Tiers Relativos integrada no Setup (`session-models.json`)
- [x] F1.2: Roteamento com Auto-Upgrade (Cascade Invisível no `PreToolUse`)
- [x] F1.3: Detecção de Aceite via Git Commit (`used/discarded`)

### 📦 Tarefas Compactadas em 2026-10-28 03:22:06 (5 itens)
- [x] F1.4: Validação E2E no Claude Code (bateria dos 4 hooks)
- [x] F1.5: Inventário de Cobertura de Pacotes (`docs/contrib/coverage.md`)
- [x] F1.6: Loop Engineer Engine CLI (`downshift loop`) com ciclo Plan -> Implement -> Sensor -> Cascade -> Accept
- [x] Correção de concorrência multi-processo com Directory Lock atômico em `dead_ends`
- [x] Injeção de `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` via `downshift setup`

### 📦 Tarefas Compactadas em 2026-10-28 17:05:02 (7 itens)
- [x] **W7.1 (3 pts):** Default Profile Table (N11) embutida no binário — mapeamento de tipos padrão (`Explore` -> small, `claude-code-guide` -> small, `statusline-setup` -> small; `general-purpose` e `Plan` mantidos), sem risco **[Removido 2026-10-28 (O8): defaults foram para o playbook do orquestrador]**
- [x] **W7.2 (2 pts):** `downshift policy init` integrado no `setup` — gera `.downshift/policy.json` com confirmação e preview
- [x] **S1 (3 pts):** Ownership Markers: arquivos inteiros gerados pelo setup (wrappers e steering dedicado) levam a frase `Written by \`downshift setup\``; `--undo` deixa intacto um arquivo sem a frase e o setup recusa sobrescrever arquivo não gerado por ele
- [x] **S2 (3 pts):** Instalação da skill `downshift-audit` (somente leitura) em `~/.claude/skills` e `~/.codex/skills` via setup, com marcador, backup, dry run e conflito em pasta que não é nossa
- [x] **W7.9 (3 pts):** Modo Economia (`economy mode` no `policy.json` ou flag `--economy`), rebaixando um tier com segurança **[Superado por O8: age só na política declarada; remoção pendente]**
- [x] **F2.1 (3 pts):** Batch Loop Runner (`downshift loop --batch=tasks.json`) para orquestração sequencial/isolada de tarefas
- [x] **F2.2 (2 pts):** Relatório Consolidado de Fábrica (`downshift loop report`) com telemetria e taxa de aceitação git
