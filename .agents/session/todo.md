# Todo — Downshift Setup Evolution & Engineering Playbooks

Plano de execução: Seção 3b em [roadmap.md](../../docs/internal/plans/roadmap.md) e [discovery-setup-wizard.md](file:///Users/tiago.boas/.gemini/antigravity/brain/cd7a6239-2038-43ef-bf3f-d34702ea2d8c/discovery-setup-wizard.md).

## Setup Evolution Waves (Cards S-W1 a S-W5)

- [ ] **S-W3 (3 SP): Catálogo dos 6 Playbooks de Engenharia & Universal Skill**
  - [ ] Criar `internal/playbook/assets/research.md` com state-machine Mermaid (Strict Read-Only)
  - [ ] Criar `internal/playbook/assets/implementation.md` com state-machine Mermaid (Clean Slice)
  - [ ] Criar `internal/playbook/assets/review.md` com state-machine Mermaid (AppSec & Quality Sensor)
  - [ ] Criar `internal/playbook/assets/tdd-verification.md` com state-machine Mermaid (Test Discipline)
  - [ ] Criar `internal/playbook/assets/docs.md` com state-machine Mermaid (Technical Documentation)
  - [ ] Criar `internal/playbook/assets/incident-rescue.md` com state-machine Mermaid (Emergency Debugging)
  - [ ] Criar `internal/playbook/assets/universal-skill.md` (Downshift Universal Skill para orquestração)
  - [ ] Atualizar `shipped()` em `internal/playbook/playbook.go` e testes em `internal/playbook/playbook_test.go`
  - [ ] Validar com `make check` e `docscheck`

- [ ] **S-W1 (2 SP): Interactive TTY Detection & Silent Fallback**
  - [ ] Implementar detecção de TTY (`isatty`) no comando `downshift setup`
  - [ ] Garantir modo não-interativo transparente para `--yes`, `--dry-run` e pipes
  - [ ] Testes unitários para execução interativa vs não-interativa

- [ ] **S-W2 (3 SP): First-Run Interview State Machine (Lipgloss TUI)**
  - [ ] Implementar máquina de estados com etapas de seleção de harness (1 a 7)
  - [ ] Coleta de presets de política (`Balanced`, `Aggressive`, `Safe`)
  - [ ] Detecção de gateways (OmniRoute / 9router) e concorrência
  - [ ] Confirmação visual de plano antes de aplicar

- [ ] **S-W4 (3 SP): Live Adapter Proof & Smoke Check**
  - [ ] Handshake in-process em tempo real (< 50ms) contra os hooks registrados
  - [ ] Checklist interativo com latência e status visual de verificação

- [ ] **S-W5 (2 SP): Aha! Moment Summary & Verification Gate**
  - [ ] Card de encerramento visual com quota e primeiro comando a rodar
  - [ ] Atualização de documentação (`README.md`, `install.md`) e gate final
