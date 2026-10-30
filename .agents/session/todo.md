# Todo — Downshift Setup Evolution & Engineering Playbooks

Plano de execução: Seção 3b em [roadmap.md](../../docs/internal/plans/roadmap.md) e [discovery-setup-wizard.md](file:///Users/tiago.boas/.gemini/antigravity/brain/cd7a6239-2038-43ef-bf3f-d34702ea2d8c/discovery-setup-wizard.md).

## Setup Evolution Waves (Cards S-W1 a S-W5)

- [x] **S-W3 (3 SP): Catálogo dos 6 Playbooks de Engenharia & Universal Skill (Merged PR #622)**
- [x] **S-W1 (2 SP): Interactive TTY Detection & Silent Fallback (Merged PR #623)**
- [x] **S-W2 (3 SP): First-Run Interview State Machine (Lipgloss TUI) (Merged PR #623)**

- [x] **S-W4 (3 SP): Live Adapter Proof & Smoke Check**
  - [x] Handshake in-process em tempo real (< 50ms) contra os hooks registrados em `harnessRegistry`
  - [x] Suporte a payloads de smoke sintéticos para cada harness (`smokePayloadFor`)
  - [x] Exibição de latência real em microssegundos no final do Setup Wizard
  - [x] Testes unitários para todos os adapters (`TestSmokeCheckAllHarnesses`)

- [ ] **S-W5 (2 SP): Aha! Moment Summary & Verification Gate**
  - [ ] Card de encerramento visual com quota e primeiro comando a rodar
  - [ ] Atualização de documentação (`README.md`, `install.md`) e gate final
