# REA: Kiro IDE e KiroCrew — Análise Binária Local

**Data:** 2026-11-10  
**Versões analisadas:**
- Kiro.app: v1.1.787 (kiroAgent extension)
- KiroCrew.app: v0.11.2 (kirocrew Python package)

## Resumo Executivo

| Harness | Tipo | Hooks PreToolUse | updatedInput.model | Roteamento de modelo |
|---------|------|------------------|-------------------|---------------------|
| **Kiro IDE** | Electron (VS Code fork) | ✅ exit 0/2 + permissionDecision:ask | ❌ Não existe | Design-time (agent specs) |
| **KiroCrew** | Electron + Python backend | ✅ Script hooks | ❌ Não existe | Config + model_scope.py |
| **Claude Code** | Reference | ✅ | ✅ updatedInput.model | Runtime rewrite |

**Conclusão:** Nem Kiro nem KiroCrew suportam `updatedInput` para reescrita de modelo em runtime. Ambos usam model binding em tempo de design/configuração.

---

## Metodologia

1. **Kiro.app**: Análise direta do código JS em `/Applications/Kiro.app/Contents/Resources/app/out/`
2. **KiroCrew.app**: Extração do `app.asar` e análise do pacote Python `kiro_crew` em `site-packages/`

---

## Kiro IDE — Análise Detalhada

### Estrutura do App

```
/Applications/Kiro.app/Contents/Resources/app/
├── out/
│   ├── main.js                    # Main process
│   ├── cli.js                     # CLI
│   └── vs/
│       ├── platform/agentHost/node/agentHostMain.js  # Hook handling
│       └── workbench/workbench.desktop.main.js
├── extensions/
│   └── kiro.kiro-agent/
│       ├── dist/extension.js      # 17MB minificado
│       └── package.json           # kiroAgentVersion: 0.66.89
└── package.json
```

### Hooks — Código Encontrado

**Arquivo:** `agentHostMain.js`

```javascript
// Hook events definidos
preToolUse:"PreToolUse",
postToolUse:"PostToolUse",
userPromptSubmitted:"UserPromptSubmit",
agentStop:"Stop"

// permissionDecision handling
permissionDecision:"ask"
permissionDecision==="ask"
permissionDecisionReason:"reason"
```

**Arquivo:** `kiro.kiro-agent/dist/extension.js`

```javascript
// hookSpecificOutput parsing
hookSpecificOutput":{"permissionDecision":"ask","permissionDecisionReason":"reason"}}
hookSpecificOutput;if(typeof n!="object"||n===null)return;
if(n.permissionDecision==="ask"){let r=n.permissionDecisionR...

// Decisões suportadas
o.decision==="block"  // Bloqueia a tool
// Apenas ask/block — não há rewrite
```

### updatedInput — Evidência

**Busca:** `grep -oE "updatedInput[^}]*" agentHostMain.js`

```javascript
updatedInput:e}):{behavior:"deny"     // Tool input (answers), não model
updatedInput:t}:{behavior:"deny"
updatedInput:{...e,answers:l}         // Apenas campo "answers"
```

**Conclusão:** `updatedInput` existe no Kiro, mas apenas para modificar **tool input** (campo `answers`), **nunca para model**.

### Model Selection — Evidência

**Arquivo:** `kiro.kiro-agent/dist/extension.js`

```javascript
// modelId é parâmetro, não algo modificável por hooks
modelId:S.string().optional(),effortLevel:S.string().optional(),input:QFo
modelId:S.string().optional(),effortLevel:S.string().optional(),sessionId:...
```

O modelo é definido via:
- Agent specs em `.kiro/agents/`
- Configurações de sessão
- **Nunca por hooks em runtime**

---

## KiroCrew — Análise Detalhada

### Estrutura do App

```
/Applications/KiroCrew.app/Contents/Resources/
├── app.asar                       # Electron frontend
└── backend-dist/
    └── kirocrew-backend-arm64/
        ├── bin/kirocrew           # Entry point Python
        └── lib/python3.12/site-packages/kiro_crew/
            ├── hooks.py           # Hook system
            ├── hook_runtime/      # Hook dispatch
            ├── model_registry.json
            ├── model_scope.py     # Model selection logic
            └── acp/               # Agent Control Protocol
                └── client_model_selection.py
```

### Hook System

**Arquivo:** `hooks.py`

```python
"""Config-driven hook system for KiroCrew's message pipeline.
Hooks intercept messages and tool calls based on rules in config.json.
Supports declarative rules and executable script hooks with timeout/sandboxing.
"""
```

**Arquivo:** `hook_runtime/hook_dispatch.py`

```python
async def fire_tool_hooks(
    hook_store: ScriptHookStore | None,
    event_title: str,
    event_tool_input: str | None = None,
    ...
) -> None:
    """Fire PreToolUse hooks for an EVENT_TOOL_CALL event.
    
    Note: For EVENT_TOOL_CALL, hooks are informational only. The tool is
    already running (auto-approved by kiro-cli), so hook results cannot
    block execution. Hook scripts can log, audit, or trigger side effects.
    """
```

### updatedInput — Busca Completa

```bash
$ grep -rn "updatedInput\|updated_input" kiro_crew/
# Resultado: 0 matches
```

**Conclusão:** KiroCrew não implementa `updatedInput` em nenhum lugar.

### Model Selection — Código Real

**Arquivo:** `model_scope.py`

```python
"""Which harness a model pin was chosen for, so one harness's pin is not sent to another.

A stored model pin -- ``agent.model`` in ``config.json``, a dashboard slot's
``model``, a kiro agent spec's ``model`` field -- records WHAT was picked and
nothing about WHERE.
"""
```

**Arquivo:** `model_registry.json` (parcial)

```json
{
  "fable-5-1m": {
    "display": "Fable 5 (1M context)",
    "window": 1000000,
    "providers": { "claude_code": "global.anthropic.claude-fable-5[1m]", "acp": "claude-fable-5" }
  },
  "opus-4.8-1m": {
    "display": "Opus 4.8 (1M context)",
    "default": true,
    "providers": { "claude_code": "global.anthropic.claude-opus-4-8[1m]", "acp": "claude-opus-4.8" }
  }
}
```

O modelo é selecionado via:
- `config.json` (agent.model)
- Dashboard slots
- Agent spec files (`model` field)
- **Nunca por hooks em runtime**

---

## Comparação com Claude Code

| Feature | Claude Code | Kiro IDE | KiroCrew |
|---------|------------|----------|----------|
| PreToolUse hook | ✅ | ✅ | ✅ |
| exit 0 (allow) | ✅ | ✅ | ✅ |
| exit 2 (block) | ✅ | ✅ | ✅ (ask) |
| hookSpecificOutput | ✅ | ✅ | ❌ |
| permissionDecision:ask | ✅ | ✅ | ❌ |
| **updatedInput** | ✅ | ❌ | ❌ |
| **updatedInput.model** | ✅ | ❌ | ❌ |
| Model via config | ✅ | ✅ | ✅ |
| Model via agent spec | N/A | ✅ | ✅ |

---

## Implicações para o Downshift

### Kiro IDE

1. **Modo:** Observation-only
2. **Capacidade:** Log tool calls, block dangerous tools
3. **Não pode:** Reescrever modelo em runtime
4. **Alternativa:** Design-time steering via `.kiro/steering/` e `.kiro/agents/`

### KiroCrew

1. **Modo:** Observation-only (hooks são "informational only" por design)
2. **Capacidade:** Audit, log, side effects
3. **Não pode:** Reescrever modelo em runtime
4. **Alternativa:** Config-based model pin via `config.json`

---

## Arquivos Analisados

### Kiro
- `/Applications/Kiro.app/Contents/Resources/app/out/vs/platform/agentHost/node/agentHostMain.js`
- `/Applications/Kiro.app/Contents/Resources/app/extensions/kiro.kiro-agent/dist/extension.js`
- `/Applications/Kiro.app/Contents/Resources/app/extensions/kiro.kiro-agent/package.json`

### KiroCrew
- `/tmp/kirocrew-rea/app/*.js` (extraído de app.asar)
- `site-packages/kiro_crew/hooks.py`
- `site-packages/kiro_crew/hook_runtime/hook_dispatch.py`
- `site-packages/kiro_crew/model_scope.py`
- `site-packages/kiro_crew/model_registry.json`
- `site-packages/kiro_crew/acp/client_model_selection.py`

---

## Recomendação Final

Para ambos Kiro e KiroCrew, o Downshift deve:

1. **Operar em modo observation-only**
2. **Logging:** Registrar tool calls para análise de custo
3. **Advisory:** Usar steering/skills para recomendar modelo por tipo de tarefa
4. **Não tentar:** Reescrita de modelo via hooks (não funciona)

O roteamento de modelo deve ser feito em **design-time**:
- Kiro: `.kiro/agents/*.yaml` com campo `model`
- KiroCrew: `config.json` agent.model ou dashboard slots
