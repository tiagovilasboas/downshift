# Spike: shell wrap em todos os harnesses

**Status:** proposed  
**Criado:** 2026-10-22  
**Autor:** Tiago Vilas Boas  
**Relacionado:** `docs/spikes/kirocrew-shell-policy.md` (A-P1 do roadmap)

---

## Pergunta

O shell wrap (compactor) já gera valor mensurável no KiroCrew (A-P1). Quais
harnesses podem adotar a mesma camada e por qual mecanismo — e onde o valor
real aparece, dado o que cada harness consegue fazer hoje?

---

## O que a arquitetura já resolve

`shellWrapDecision` em `cmd/downshift/shellrewrite.go` é harness-agnóstico.
Recebe `harness string` e `command string`, decide em cima da família e da
capability table. O adaptador só precisa implementar três métodos:

```go
type shellEvent interface {
    ShellCommand()  (string, bool)     // "é shell, e o comando é X"
    ShellRewrite(cmd string) (any, error)  // wire output que substitui o cmd
    ShellBlocked(cmd string) bool          // as deny/ask rules do usuário
}
```

Hoje só o Claude Code implementa isso (`internal/adapters/claudecode/shell.go`).
A barreira por harness é uma linha em `capabilities.go`
(`CanRewriteShellCommand: true`) mais implementar os três métodos no
adaptador — mas `CanRewriteShellCommand` exige que o protocolo do harness
suporte `updated_input` / `updatedInput` em resposta ao preToolUse.

---

## Matriz por harness

### Claude Code — `CanRewriteShellCommand: true` ✅ já funciona

**Mecanismo:** preToolUse retorna `updatedInput.command`. Confirmado ao vivo.  
**Shell tool:** `Bash`  
**Valor:** já ativo — maior fonte de ganho em sessões de código (go test, pytest, jest).

---

### KiroCrew — `CanRewriteShellCommand: false`, piloto de advise (A-P1)

**Mecanismo disponível:** exit 2 com stderr para o modelo (advise, não rewrite).
Não existe `updated_input` no protocolo KiroCrew — o hook responde allow/block,
o modelo lê o stderr e re-emite com o comando embrulhado.  
**Shell tool:** `execute_bash`  
**Valor:** toda sessão de coding no KiroCrew que roda testes, builds ou lint.
Um `go test ./...` no KiroCrew gera ~100k tokens de log; o compactor reduz
para ~17k. Com A-P1 isso acontece sem intervenção manual.  
**Barreira:** o modelo precisa ler o stderr e obedecer. A-P2 valida isso ao vivo.
Se o modelo ignorar ou o turn morrer, reverte.  
**`CanAdviseShellWrap` (nova cap, não `CanRewriteShellCommand`):** colocada em
`KiroCrewCaps` por A-P1 — separada para não confundir o caminho de rewrite
real com o de advise.

---

### Cursor — `CanRewriteShellCommand: false` ❓ não verificado

**Mecanismo disponível:** `RewritePreservesInput: true` — preToolUse aceita
`updated_input.model`. **Não está provado** que `updated_input.command` também
é lido.  
**Shell tool:** provavelmente `run_terminal_cmd` (nome observado em payloads
externos; não confirmado em payload downshift real).  
**Barreira principal:** nenhum payload com `tool_name = run_terminal_cmd` foi
capturado vindo do hook do Cursor. Sem payload real não é possível:
1. Confirmar o `tool_name` exato
2. Confirmar que `updated_input.command` sobrevive ao preToolUse
3. Confirmar que o plano pago aplica o rewrite (contas free/legacy ignoram
   `updated_input.model` silenciosamente — o mesmo pode ocorrer com command)

**Ação antes de codar:** capturar um payload Cursor com `tool_name` de shell e
verificar se `updated_input.command` resulta num retry com o comando alterado.
Registrar em `docs/evidence/`.  
**Valor potencial:** Cursor é o segundo harness mais usado. Se o rewrite
funcionar, o ganho é imediato nos mesmos runners (go test, pytest, jest).

---

### Codex — `CanRewriteShellCommand: false` ❓ não verificado

**Mecanismo disponível:** `updatedInput.model` + `reasoning_effort` funcionam
(`RewritePreservesInput: true`, `CanApplyEffort: true`). `updatedInput.command`
é plausível mas não verificado.  
**Shell tool:** `shell` ou `computer` (nome do Codex para shell); não confirmado
em payload downshift real.  
**Barreira:** igual ao Cursor — sem payload capturado com tool de shell não se
sabe o `tool_name` nem se `updated_input.command` funciona.  
**Ação antes de codar:** idem Cursor.

---

### OpenCode — `CanRewriteShellCommand: false` ❌ protocolo diferente

**Mecanismo disponível:** o protocolo OpenCode é `{action:"route", model:{...}}`,
não `updated_input`. `RewritePreservesInput: false`.  
**Shell tool:** desconhecido.  
**Barreira:** o protocolo não tem um campo de command override no wire format
documentado. Mesmo que existisse, o harness aplica uma resposta estruturada
— não há caminho direto de rewrite de shell provado.  
**Ação:** nenhuma agora. Revisitar quando o protocolo OpenCode for melhor
documentado (parte do work de D-1/D-2 OpenRouter).

---

### Antigravity — `CanRewriteShellCommand: false` ❌ sem evidência

**Mecanismo disponível:** `RewritePreservesInput: true` mas só `model` no wire
(aliases flash_lite/flash/pro). Nenhuma menção a shell tool nos payloads ou docs
disponíveis.  
**Barreira:** harness de automação visual; o conceito de "shell command" não
corresponde ao modelo de uso. Fora do escopo prático.

---

## Onde está o valor real

```
Harness          | Ganho mensurável    | Mecanismo       | Pré-requisito
-----------------|---------------------|-----------------|------------------------
Claude Code      | ✅ já ativo         | updatedInput    | —
KiroCrew (A-P1)  | alto (100k→17k/run) | exit 2 + stderr | A-P1 implementado + A-P2 vivo
Cursor           | alto (mesmo volume) | updated_input?  | payload capturado + honrado
Codex            | médio              | updated_input?  | payload capturado + honrado
OpenCode         | desconhecido       | protocolo ?     | redesign de protocolo
Antigravity      | irrelevante        | sem shell tool  | fora do escopo
```

O ganho maior depois do Claude Code está no **KiroCrew** (A-P1) e no **Cursor**
— ambos os harnesses que os devs usam para sessões longas de coding com testes.
Codex é menor porque as sessões são mais curtas.

---

## Como o design agnóstico gera valor

A separação que já existe protege o investimento:

1. **Uma capability, dois comportamentos.** `CanRewriteShellCommand` = rewrite
   in-place (Claude Code). `CanAdviseShellWrap` = advise + retry (KiroCrew,
   e potencialmente qualquer harness exit-2). Mesma decisão de família,
   mecanismo de entrega diferente.

2. **Nenhum adapter precisa reimplementar a lógica.** `shellWrapDecision` já
   resolve kill-switch, config, política, capability e deny/ask rules. O adapter
   só expõe `ShellCommand()`, `ShellRewrite()` ou `ShellBlocked()`.

3. **A barreira de custo por harness é 1 capability flag + 3 métodos no adapter.**
   Para Cursor: `CanRewriteShellCommand: true` + implementar `shell.go` no
   adapter cursor. Estimativa: 2 pts — desde que o payload prove que funciona.

4. **O sensor já existe.** A-P3 conta bloqueios na telemetria. Com isso, dá para
   medir por harness quantas vezes o wrap foi sugerido e quantas foram honradas
   — sem mudar o core.

---

## Roadmap de expansão (complementa o Trilha A)

| Fase | Pts | O que é | Pré-requisito |
|------|-----|---------|---------------|
| A-P1 | 2 | KiroCrew advise (exit 2 + stderr) | nada |
| A-P2 | 1 | Validação ao vivo: modelo lê stderr e retry roda embrulhado | A-P1 instalado |
| A-Cursor | 2 | Cursor `CanRewriteShellCommand`: implement `shell.go` + cap flag | payload capturado provando `updated_input.command` + plano honrado |
| A-Codex | 2 | Codex `CanRewriteShellCommand`: implement `shell.go` + cap flag | payload capturado + honrado |
| A-P3 | 2 | Telemetria de bloqueios por harness (quantas vezes o retry disparou) | A-P2 segurou |
| A-P4 | 2 | Child-end tokens KiroCrew de postToolUse real | payload capturado (não A-P1) |
| A-P5 | 3 | `updated_input.command` no Cursor e Codex (uma mudança por harness) | A-Cursor e A-Codex + prova ao vivo |

**A captura do payload Cursor pode correr em paralelo com A-P1.** Não desbloqueie
o rewrite Cursor sem o payload — uma flag sem evidência ao vivo pode descartar
ou rejeitar o comando silenciosamente.

---

## Fora do escopo

- OpenCode e Antigravity: protocolo incompatível ou sem shell tool
- Harnesses que não controlam `updated_input` (exit-0-only)
- Compressão de output que não é shell (context, DCI): ortogonal a este spike
- Ganhos que não dependem de shell wrap (roteamento de modelo, DCI): outros spikes

---

## Acceptance

Para cada harness que sair de "não verificado" para "ativo":

1. Payload capturado ao vivo com `tool_name` de shell e `updated_input.command`
   alterado — registrado em `docs/evidence/`.
2. `go test ./...` no adapter: block em família comprimível, allow em passthrough
   e em harness sem a cap.
3. Uma sessão real com o compressor ativo: log menor que sem ele.
