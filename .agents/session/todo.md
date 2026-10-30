# Todo — Downshift Setup Evolution & Engineering Playbooks

Plano de execução: Seção 3b em [roadmap.md](../../docs/internal/plans/roadmap.md) e [discovery-setup-wizard.md](file:///Users/tiago.boas/.gemini/antigravity/brain/cd7a6239-2038-43ef-bf3f-d34702ea2d8c/discovery-setup-wizard.md).

## Setup Evolution Waves (Cards S-W1 a S-W5)

- [x] **S-W3 (3 SP): Catálogo dos 6 Playbooks de Engenharia & Universal Skill (Merged PR #622)**
  - [x] Criar `internal/playbook/assets/research.md` com state-machine Mermaid (Strict Read-Only)
  - [x] Criar `internal/playbook/assets/implementation.md` com state-machine Mermaid (Clean Slice)
  - [x] Criar `internal/playbook/assets/review.md` com state-machine Mermaid (AppSec & Quality Sensor)
  - [x] Criar `internal/playbook/assets/tdd-verification.md` com state-machine Mermaid (Test Discipline)
  - [x] Criar `internal/playbook/assets/docs.md` com state-machine Mermaid (Technical Documentation)
  - [x] Criar `internal/playbook/assets/incident-rescue.md` com state-machine Mermaid (Emergency Debugging)
  - [x] Criar `internal/playbook/assets/universal-skill.md` (Downshift Universal Skill para orquestração)
  - [x] Atualizar `shipped()` em `internal/playbook/playbook.go` e testes em `internal/playbook/playbook_test.go`
  - [x] Validar com `make check` e `docscheck`

- [x] **S-W1 (2 SP): Interactive TTY Detection & Silent Fallback**
  - [x] Implementar detecção de TTY (`isatty`) no comando `downshift setup`
  - [x] Suporte a `--no-tui` e garantia de modo não-interativo transparente para `--yes`, `--dry-run` e pipes
  - [x] Testes unitários para execução interativa vs não-interativa (`TestIsInteractiveTerminal`)

- [x] **S-W2 (3 SP): First-Run Interview State Machine (Lipgloss TUI)**
  - [x] Reordenar `setupTargets` seguindo a relevância da comunidade (Claude, Cursor, OpenCode, Codex, Antigravity, KiroCrew)
  - [x] Coleta de presets de política (`Balanced`, `Aggressive`, `Safe`)
  - [x] Detecção e relato de gateways companheiros (OmniRoute / 9router)
  - [x] Confirmação visual de plano antes de aplicar e card de encerramento com dicas de início rápido

- [ ] **S-W4 (3 SP): Live Adapter Proof & Smoke Check**
  - [ ] Handshake in-process em tempo real (< 50ms) contra os hooks registrados
  - [ ] Checklist interativo com latência e status visual de verificação

- [ ] **S-W5 (2 SP): Aha! Moment Summary & Verification Gate**
  - [ ] Card de encerramento visual com quota e primeiro comando a rodar
  - [ ] Atualização de documentação (`README.md`, `install.md`) e gate final
