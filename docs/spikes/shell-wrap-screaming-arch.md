# Spike: shell wrap — screaming architecture

**Status:** proposed  
**Criado:** 2026-10-22  
**Autor:** Tiago Vilas Boas  
**Predecessores:** `kirocrew-shell-policy.md`, `shell-wrap-harness-matrix.md`

---

## Problema

O shell wrap hoje tem dois problemas de organização:

1. **A decisão e o protocolo estão misturados em `cmd/downshift/shellrewrite.go`.**
   `shellWrapDecision` (política pura) e `shellRewrite` (entrega de wire) vivem
   no mesmo arquivo que conhece tanto `execwrap` quanto o tipo concreto do evento.

2. **O adapter Claude Code implementa `shell.go` mas não há contrato explícito
   para os outros adapters adotarem.** A interface `shellEvent` está em
   `cmd/downshift/`, não em `internal/` — invisível para quem escreve um adapter
   novo.

O resultado: adicionar Cursor ou KiroCrew advise exige entender o `cmd/` inteiro
antes de tocar qualquer linha de adapter.

---

## Objetivo

Extrair uma camada `internal/shellwrap/` que grite o que ela faz:

- **`internal/shellwrap/`** — domínio puro: política, contrato de adapter, tipo
  de resultado. Zero dependência de protocolo de harness.
- **`internal/adapters/<harness>/shell.go`** — implementação da interface para
  aquele harness. Só ele conhece o wire format.
- **`cmd/downshift/shellrewrite.go`** — orquestrador fino: chama o domínio,
  entrega o resultado ao hookrun. Sem lógica de política.

---

## Estrutura proposta

```
internal/
  shellwrap/                        ← DOMÍNIO (screaming: "eu decido se embrulha")
    doc.go                          ← package doc
    policy.go                       ← Decide(harness, command, blocked) Decision
    adapter.go                      ← interface ShellAdapter + tipo AdviseOutput
    caps.go                         ← CanRewrite / CanAdvise lidos de core.CapsFor
    policy_test.go
    adapter_test.go

  adapters/
    claudecode/
      shell.go                      ← já existe; importa shellwrap.ShellAdapter
    cursor/
      shell.go                      ← novo; stub até payload capturado
    kirocrew/
      shell.go                      ← novo (A-P1); implement AdviseAdapter
    codex/
      shell.go                      ← novo; stub até payload capturado

cmd/downshift/
  shellrewrite.go                   ← orquestrador: wiring shellwrap + adapter
```

---

## Contratos

### `internal/shellwrap/adapter.go`

```go
package shellwrap

// RewriteAdapter is implemented by adapters whose harness supports
// in-place command replacement (updatedInput / updated_input).
// Claude Code today; Cursor and Codex when payload is proven.
type RewriteAdapter interface {
    // ShellCommand returns the command and true when this call is the
    // harness's shell tool. The shared runner never routes shell calls.
    ShellCommand() (command string, isShell bool)
    // ShellRewrite returns the wire output that replaces the command,
    // copying every sibling field of tool_input unchanged.
    ShellRewrite(command string) (any, error)
    // ShellBlocked reports whether the harness's own deny/ask rules
    // forbid rewriting this command. Any rule-read error is true (fail closed).
    ShellBlocked(command string) bool
}

// AdviseAdapter is implemented by adapters whose harness cannot rewrite
// in place but can show the model one line of stderr (exit 2).
// KiroCrew today; any exit-2 harness later.
type AdviseAdapter interface {
    ShellCommand() (command string, isShell bool)
    ShellBlocked(command string) bool
    // AdviseMessage returns the stderr line to show the model when the
    // compactor suggests a retry. The shared runner computes the wrapped
    // command; the adapter only formats the message.
    AdviseMessage(wrappedCommand string) string
}

// Result is what the shared runner hands back to hookrun.
type Result struct {
    IsShell   bool
    // Rewrite is non-nil when the command should be replaced in place.
    Rewrite   any
    // Advise is non-empty when the runner should exit 2 with this message.
    Advise    string
    // Decision carries the family and reason for telemetry.
    Decision  execwrap.Decision
}
```

### `internal/shellwrap/policy.go`

```go
package shellwrap

// Handle is the single entry point. It replaces both shellWrapDecision
// and shellRewrite in cmd/. Returns a Result for hookrun to act on.
//
// - If ev does not implement RewriteAdapter or AdviseAdapter: Result{}.
// - If it is a shell tool but the policy says allow: Result{IsShell: true}.
// - If policy says rewrite: Result{IsShell: true, Rewrite: <wire>}.
// - If policy says advise:  Result{IsShell: true, Advise: <msg>}.
func Handle(harness string, ev any) Result {
    caps, ok := core.CapsFor(harness)
    if !ok {
        return Result{}
    }
    // Try rewrite path first (in-place, CanRewriteShellCommand).
    if rw, ok := ev.(RewriteAdapter); ok {
        return handleRewrite(harness, caps, rw)
    }
    // Try advise path (exit-2, CanAdviseShellWrap).
    if ad, ok := ev.(AdviseAdapter); ok {
        return handleAdvise(harness, caps, ad)
    }
    return Result{}
}
```

### `cmd/downshift/shellrewrite.go` (após refactor — slim)

```go
// shellRewrite delegates entirely to the shellwrap domain.
// hookrun calls this; it never inspects the result beyond acting on it.
func shellRewrite[E any](harness string, ev E) (out any, isShell bool) {
    r := shellwrap.Handle(harness, ev)
    if !r.IsShell {
        return nil, false
    }
    if r.Rewrite != nil {
        return r.Rewrite, true
    }
    // Advise: hookrun returns exit 2 and writes r.Advise to stderr.
    // (hookrun reads result.Advise; shellrewrite only passes it through.)
    return shellwrap.AdviseResult(r), true
}
```

---

## Adapters por harness

### `internal/adapters/claudecode/shell.go` (move, não muda)

Já implementa `RewriteAdapter`. Mover import de `shellEvent` para
`shellwrap.RewriteAdapter`. Zero mudança de comportamento.

### `internal/adapters/kirocrew/shell.go` (novo — A-P1)

```go
package kirocrew

import "fmt"

// shellToolName is KiroCrew's shell tool. Confirmed on live payload YYYY-MM-DD.
const shellToolName = "execute_bash"

func (ev Event) ShellCommand() (string, bool) {
    // parse tool_input.command from ev
}

func (ev Event) ShellBlocked(command string) bool {
    return false // KiroCrew has no deny/ask rules on shell today
}

func (ev Event) AdviseMessage(wrapped string) string {
    return fmt.Sprintf("Retry as: %s", wrapped)
}
```

Implementa `AdviseAdapter`, não `RewriteAdapter` — KiroCrew não tem
`updated_input`.

### `internal/adapters/cursor/shell.go` (stub — até payload)

```go
package cursor

// shellToolName is unconfirmed. Set after a live payload is captured
// in docs/evidence/cursor-shell-payload-YYYY-MM-DD.json.
// const shellToolName = "run_terminal_cmd" // UNVERIFIED

// ShellCommand always returns false until tool_name is confirmed.
func (ev Event) ShellCommand() (string, bool) { return "", false }
func (ev Event) ShellRewrite(cmd string) (any, error) { return nil, nil }
func (ev Event) ShellBlocked(cmd string) bool { return false }
```

Implementa `RewriteAdapter` como stub fail-open. A flag
`CanRewriteShellCommand: false` em `CursorCaps` garante que `Handle` nunca
chega ao rewrite mesmo que o adapter responda. A flag vira `true` quando o
payload provar que funciona.

### `internal/adapters/codex/shell.go` (stub — igual)

Mesmo padrão do Cursor.

---

## Capability table (mudanças em `capabilities.go`)

```go
// nova cap — apenas KiroCrew por agora
type HarnessCapabilities struct {
    ...
    // CanAdviseShellWrap means the harness can show the model one line of
    // stderr (exit 2) suggesting a retry with the wrapped command. Distinct
    // from CanRewriteShellCommand (in-place replacement).
    CanAdviseShellWrap bool
}

KiroCrewCaps = HarnessCapabilities{
    ...
    CanAdviseShellWrap: true,   // A-P1
}
```

---

## Estrutura de testes (screaming)

```
internal/shellwrap/
  policy_test.go
    TestHandle_RewriteAdapter_compressingFamily  → Rewrite não-nil
    TestHandle_RewriteAdapter_passthroughFamily  → Result{IsShell:true, Rewrite:nil}
    TestHandle_AdviseAdapter_compressingFamily   → Advise não-vazio
    TestHandle_AdviseAdapter_EXEC0              → Result{IsShell:true} (allow)
    TestHandle_nonShellTool                     → Result{}
    TestHandle_unknownHarness                   → Result{}

internal/adapters/kirocrew/
  shell_test.go
    TestShellCommand_execute_bash               → true
    TestShellCommand_otherTool                  → false
    TestAdviseMessage_format                    → "Retry as: /path/compact -- ..."

internal/adapters/claudecode/
  shell_test.go  (já existe; agora implementa shellwrap.RewriteAdapter)

internal/adapters/cursor/
  shell_test.go
    TestShellCommand_alwaysFalse_untilPayload   → false (stub)
```

---

## Migração (não quebra nada)

1. Criar `internal/shellwrap/` com `adapter.go` + `policy.go` + `caps.go`.
2. Mover `shellWrapDecision` + `shellRewrite` para `shellwrap.Handle` — mesma
   lógica, novo pacote.
3. Atualizar `internal/adapters/claudecode/shell.go`: trocar `shellEvent` por
   `shellwrap.RewriteAdapter`. Interface idêntica — zero mudança de comportamento.
4. Atualizar `cmd/downshift/shellrewrite.go`: delegar para `shellwrap.Handle`.
5. Adicionar `CanAdviseShellWrap` em `capabilities.go` (falso em todos).
6. Implementar `internal/adapters/kirocrew/shell.go` + virar `CanAdviseShellWrap:
   true` em `KiroCrewCaps` — esse é A-P1.
7. Adicionar stubs em cursor e codex (fail-open, caps falsas).

Cada passo é um commit atômico. O `go test ./...` fica verde em cada um.

---

## O que grita nesta arquitetura

- Abrir `internal/shellwrap/` → você sabe imediatamente que é a política de
  embrulho de shell, isolada de qualquer harness.
- Abrir `internal/adapters/cursor/shell.go` → você sabe que é o adapter de
  shell do Cursor, e o stub diz claramente que está esperando payload.
- Abrir `cmd/downshift/shellrewrite.go` → orquestrador fino, sem lógica.
- A capability table continua como fonte de verdade: um harness sem a cap nunca
  chega ao adapter.

---

## Estimativa

| Passo | Pts | Arquivo principal |
|-------|-----|-------------------|
| Criar `internal/shellwrap/` (extração) | 3 | shellwrap/policy.go + adapter.go |
| Migrar claudecode/shell.go | 1 | adapters/claudecode/shell.go |
| Slim cmd/shellrewrite.go | 1 | cmd/downshift/shellrewrite.go |
| kirocrew/shell.go + cap (A-P1) | 2 | adapters/kirocrew/shell.go |
| Stubs cursor + codex | 1 | adapters/cursor/shell.go, codex/shell.go |
| **Total** | **8** | |

Pode rodar em 2 waves: extração (5 pts, sem mudança de comportamento) + A-P1
(3 pts, o primeiro comportamento novo). Os stubs andam junto da extração.

---

## Fora do escopo

- Antigravity e OpenCode: sem shell tool confirmado.
- Telemetria de bloqueios (A-P3): depende de A-P2 ao vivo.
- Promoção de Cursor/Codex para `CanRewriteShellCommand: true`: depende de
  payload capturado e honrado ao vivo.
