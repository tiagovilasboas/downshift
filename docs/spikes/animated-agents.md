# Spike: Animated Agents para Downshift Dashboard

> Inspirado no [savvy-progress](https://github.com/wolvern666/savvy-progress) do claude-kit

## Objetivo

Criar agentes visuais animados que representam o estado de cada subagent no dashboard:
- **Idle**: esperando trabalho
- **Working**: executando tarefa (animação principal)
- **Done**: concluído com sucesso
- **Error**: falhou

## Referência: Savvy Progress

O savvy-progress usa **tiers com animações temáticas**:
- 🦀 Crab: caminha enquanto trabalha
- 🧑‍🚀 Astronaut: flutua
- 🔍 Detective: varre com lupa
- 🔧 Engineer: gira chave inglesa
- 👨‍🍳 Chef: joga omelete
- 🏃 Racer: corre com bandeira

Respeita `prefers-reduced-motion`.

## Proposta para Downshift

### Personagens por Harness

| Harness | Personagem | Animação Working | Cor Accent |
|---------|------------|------------------|------------|
| claude-code | 🤖 Robot | Olhos piscando, antena girando | `#F97316` (orange) |
| cursor | 🖱️ Cursor | Clicando, sparkles | `#8B5CF6` (purple) |
| codex | 📜 Scroll | Desenrolando, escrevendo | `#22C55E` (green) |
| kirocrew | 👻 Ghost | Flutuando, brilhando | `#06B6D4` (cyan) |
| antigravity | 🚀 Rocket | Propulsão, estrelas | `#EF4444` (red) |
| opencode | 🔓 Lock | Destravando | `#FBBF24` (yellow) |

### Estados Visuais

```
┌─────────────────────────────────────────────┐
│  IDLE          WORKING         DONE   ERROR │
│                                              │
│   😴            🏃💨            ✅      ❌   │
│  grayscale    animated+glow   green   red   │
│  opacity:0.5  full opacity    pulse   shake │
└─────────────────────────────────────────────┘
```

### Estrutura de Componentes (Svelte 5)

```
dashboard/src/features/agents/
├── AnimatedAgent.svelte      # Container com estado
├── characters/
│   ├── Robot.svelte          # claude-code
│   ├── Cursor.svelte         # cursor  
│   ├── Scroll.svelte         # codex
│   ├── Ghost.svelte          # kirocrew
│   ├── Rocket.svelte         # antigravity
│   └── Lock.svelte           # opencode
├── animations/
│   └── agent-animations.ts   # Motion configs
└── index.ts
```

### Stack Técnica

- **Svelte 5**: componentes reativos
- **Motion**: animações (já instalado no projeto)
- **SVG inline**: personagens vetoriais escaláveis
- **CSS Variables**: cores do tema Downshift
- **prefers-reduced-motion**: respeitar acessibilidade

---

## Roadmap

### Fase 1: Fundação (3 pontos)
- [ ] T1.1: Criar `AnimatedAgent.svelte` com prop `harness` e `state`
- [ ] T1.2: Implementar primeiro personagem (Ghost/kirocrew) como prova de conceito
- [ ] T1.3: Integrar no `LiveAgent.svelte` existente

### Fase 2: Personagens (5 pontos)
- [ ] T2.1: Robot (claude-code) com animação de antena
- [ ] T2.2: Cursor com click sparkles
- [ ] T2.3: Scroll desenrolando
- [ ] T2.4: Rocket com propulsão
- [ ] T2.5: Lock destravando

### Fase 3: Polish (3 pontos)
- [ ] T3.1: Transições entre estados (idle→working→done)
- [ ] T3.2: Reduced-motion: versões estáticas
- [ ] T3.3: Fallback genérico para harnesses desconhecidos

### Fase 4: Extras (2 pontos)
- [ ] T4.1: Tooltip com info do agent no hover
- [ ] T4.2: Som opcional (toggle nas settings)

**Total: 13 story points**

---

## Time

| Papel | Quem | Responsabilidade |
|-------|------|------------------|
| **Orchestrator** | senior-staff-ai-finops | Coordenação, priorização, integração |
| **Implementer** | senior-staff-fullstack | Componentes Svelte, Motion, SVG |
| **Reviewer** | senior-staff-appsec | Segurança (CSP, XSS em SVG inline) |
| **QA** (opt-in) | senior-staff-qa | Testes visuais, reduced-motion |

---

## Loop Engineer

### Ciclo por Task

```
SENSE → PLAN → ACT → VERIFY → COMMIT
  │                              │
  └──────────────────────────────┘
```

1. **SENSE**: Ler estado atual do dashboard, verificar dependências
2. **PLAN**: Definir arquivos a criar/modificar, acceptance criteria
3. **ACT**: Implementar componente/animação
4. **VERIFY**: `pnpm svelte-check`, preview visual no browser
5. **COMMIT**: Commit atômico em inglês, CI verde

### Checkpoints Obrigatórios

- [ ] Após cada personagem: screenshot do estado working
- [ ] Após Fase 1: PR com Ghost funcionando
- [ ] Após Fase 2: PR com todos os personagens
- [ ] Após Fase 3: PR final com polish

### Limites

- Máximo 400 linhas de diff por PR
- Máximo 1h sem checkpoint
- CI verde antes de avançar fase

---

## Decisões Técnicas

### SVG vs Canvas vs Lottie

| Opção | Prós | Contras | Decisão |
|-------|------|---------|---------|
| **SVG inline** | Escalável, themeable, pequeno | Complexidade manual | ✅ Escolhido |
| Canvas | Performance, controle | Não escalável, complexo | ❌ |
| Lottie | Animações ricas | Bundle size, After Effects | ❌ |

### Motion vs CSS Animations

| Opção | Prós | Contras | Decisão |
|-------|------|---------|---------|
| **Motion** | API declarativa, já instalado | Dependência | ✅ Escolhido |
| CSS @keyframes | Zero deps, performance | Verboso, menos controle | Fallback |

---

## Próximos Passos

1. **Agora**: Aprovar spike e começar T1.1
2. **Hoje**: Ter Ghost animado funcionando (Fase 1)
3. **Amanhã**: Completar personagens principais (Fase 2)

---

## Referências

- [savvy-progress](https://github.com/wolvern666/savvy-progress)
- [Motion for Svelte](https://motion.dev/docs/svelte-quick-start)
- [SVG Animation Guide](https://css-tricks.com/guide-svg-animations-smil/)
- Downshift brand: `#0A0A0A` bg, `#1A1A1A` surface, `#22C55E` accent
