# Downshift Agent Framework — Spike & Architecture

**Author:** Tiago Vilas Boas
**Date:** 2026-11-02
**Status:** DRAFT — Spike for evaluation
**REA Source:** [Ruflo/claude-flow](https://github.com/ruvnet/claude-flow)

---

## Executive Summary

This document proposes evolving Downshift from a **model routing system** into a
**full agent orchestration framework**, taking inspiration from Ruflo's
architecture while maintaining Downshift's core strengths: isolation, security,
and cost visibility.

The key insight: instead of refactoring the Go core, we add an **orchestration
layer** on top of the existing MCP tools (`delegate`, `collect`, etc.), bringing
Ruflo's coordination patterns without breaking what already works.

---

## 1. Current State Analysis

### 1.1 Downshift Today

```
┌─────────────────────────────────────────────────────────┐
│  Orchestrator (User's Session)                          │
│  - Calls delegate() with task + agent + mode            │
│  - Collects results with collect()                      │
│  - Reviews diffs with diff()                            │
└─────────────────────────────────────────────────────────┘
         │
         │  MCP Protocol
         ▼
┌─────────────────────────────────────────────────────────┐
│  Downshift MCP Server                                   │
│  - delegate: starts workers (Claude CLI / Codex CLI)    │
│  - collect: reads worker results                        │
│  - cancel, diff, send, steer, release                   │
└─────────────────────────────────────────────────────────┘
         │
         │  Process spawn (--strict-mcp-config)
         ▼
┌─────────────────────────────────────────────────────────┐
│  Worker Process (isolated)                              │
│  - Read-only or worktree-write mode                     │
│  - No access to user's MCP servers                      │
│  - Cannot spawn other workers                           │
└─────────────────────────────────────────────────────────┘
```

**Strengths:**
- Clean isolation model
- Cost tracking per worker
- Security (workers can't escalate)
- Works with Claude Code and Codex

**Limitations:**
- Flat hierarchy only (parent → workers)
- No coordination between workers
- No shared state/memory
- Agent definitions are ad-hoc prompts

### 1.2 Ruflo Today

```
┌─────────────────────────────────────────────────────────┐
│  UnifiedSwarmCoordinator                                │
│  - 15 specialized agents in hierarchical mesh           │
│  - Domain-based routing (security, core, integration)   │
│  - Consensus (Raft, Gossip, Byzantine)                  │
│  - Message bus for inter-agent communication            │
└─────────────────────────────────────────────────────────┘
         │
         │  Event-driven coordination
         ▼
┌─────────────────────────────────────────────────────────┐
│  Agent Pool (per domain)                                │
│  - Queen Coordinator (agent-1)                          │
│  - Security Domain (agents 2-4)                         │
│  - Core Domain (agents 5-9)                             │
│  - Integration Domain (agents 10-12)                    │
│  - Support Domain (agents 13-15)                        │
└─────────────────────────────────────────────────────────┘
         │
         │  Shared Memory (coordination namespace)
         ▼
┌─────────────────────────────────────────────────────────┐
│  Memory Service                                         │
│  - Key-value coordination store                         │
│  - swarm/[agent]/status, swarm/[agent]/progress         │
│  - Namespace isolation                                  │
└─────────────────────────────────────────────────────────┘
```

**Ruflo Patterns Worth Adopting:**
1. Domain-based agent organization
2. Typed agent definitions (SKILL.md with frontmatter)
3. Memory coordination protocol
4. Topology selection (star, mesh, hierarchical)
5. Task lifecycle (pending → queued → assigned → in-progress → completed)

---

## 2. Proposed Architecture

### 2.1 Layer Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│  Layer 4: Orchestration Skills                                       │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │
│  │ coordinator.md  │  │ swarm-init.md   │  │ topology.md     │      │
│  │ (main skill)    │  │ (init swarm)    │  │ (select topo)   │      │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘      │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 3: Agent Definitions                                          │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌─────────────┐  │
│  │ coder   │ │ tester  │ │ reviewer │ │ debugger │ │ researcher  │  │
│  └─────────┘ └─────────┘ └──────────┘ └──────────┘ └─────────────┘  │
│  ┌────────────┐ ┌──────────┐ ┌────────────┐ ┌──────────┐ ┌────────┐ │
│  │ refactor   │ │ security │ │ performance│ │ architect│ │documenter│
│  └────────────┘ └──────────┘ └────────────┘ └──────────┘ └────────┘ │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 2: Coordination Engine (NEW — Go)                             │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  internal/orchestration/                                     │    │
│  │  ├── coordinator.go    # Swarm coordination logic            │    │
│  │  ├── memory.go         # Shared state (JSON file or SQLite)  │    │
│  │  ├── topology.go       # Star/mesh/hierarchical selection    │    │
│  │  ├── registry.go       # Agent registry & capabilities       │    │
│  │  └── domain.go         # Domain-based routing                │    │
│  └─────────────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 1: Downshift Core (UNCHANGED)                                 │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │  internal/workers/                                           │    │
│  │  delegate, collect, cancel, diff, send, steer, release       │    │
│  │  (existing MCP tools — no changes needed)                    │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 Component Responsibilities

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **Coordinator** | `internal/orchestration/coordinator.go` | Manages swarm lifecycle, routes tasks to domains |
| **Memory** | `internal/orchestration/memory.go` | Shared state file (`.downshift/swarm/state.json`) |
| **Topology** | `internal/orchestration/topology.go` | Selects star/mesh based on task characteristics |
| **Registry** | `internal/orchestration/registry.go` | Loads agent definitions from `agents/*.md` |
| **Domain Router** | `internal/orchestration/domain.go` | Routes tasks to appropriate agent domain |

### 2.3 Data Flow

```
User Request
     │
     ▼
┌────────────────────────────────────────────────┐
│  Coordinator reads task                        │
│  1. Parse requirements                         │
│  2. Select topology (star for simple,          │
│     mesh for complex parallel)                 │
│  3. Route to domain(s)                         │
└────────────────────────────────────────────────┘
     │
     ▼
┌────────────────────────────────────────────────┐
│  Memory: Write initial state                   │
│  {                                             │
│    "swarm_id": "swarm-abc123",                 │
│    "status": "running",                        │
│    "topology": "star",                         │
│    "workers": {}                               │
│  }                                             │
└────────────────────────────────────────────────┘
     │
     ▼
┌────────────────────────────────────────────────┐
│  delegate() calls (existing MCP)               │
│  - Spawns isolated worker processes            │
│  - Each worker gets agent definition as prompt │
│  - Workers write status to memory file         │
└────────────────────────────────────────────────┘
     │
     ▼
┌────────────────────────────────────────────────┐
│  Coordinator polls memory                      │
│  - Reads worker status updates                 │
│  - Detects completion/failure                  │
│  - Coordinates dependent tasks                 │
└────────────────────────────────────────────────┘
     │
     ▼
┌────────────────────────────────────────────────┐
│  collect() results → integrate → report        │
└────────────────────────────────────────────────┘
```

---

## 3. Agent Definitions (10 Agents)

### 3.1 Agent Taxonomy

| # | Agent | Domain | Mode | Tier | Primary Capability |
|---|-------|--------|------|------|-------------------|
| 1 | **coder** | core | write | small/mid | Implementation, TDD |
| 2 | **tester** | quality | read | small | Run test suites, coverage |
| 3 | **reviewer** | quality | read | mid | Code review, quality |
| 4 | **debugger** | quality | read | mid | Test debugging, E2E, flaky |
| 5 | **researcher** | core | read | small | Code analysis, patterns |
| 6 | **refactor** | core | write | mid | Refactoring, cleanup |
| 7 | **architect** | core | read | mid/frontier | Design, decisions |
| 8 | **security** | security | read | mid/frontier | Security audit |
| 9 | **performance** | performance | read | mid | Profiling, optimization |
| 10 | **documenter** | support | write | small | Documentation |

### 3.2 Domain Organization (from Ruflo)

```go
// internal/orchestration/domain.go

type AgentDomain string

const (
    DomainCore        AgentDomain = "core"        // coder, researcher, refactor, architect
    DomainQuality     AgentDomain = "quality"     // tester, reviewer, debugger
    DomainSecurity    AgentDomain = "security"    // security
    DomainPerformance AgentDomain = "performance" // performance
    DomainSupport     AgentDomain = "support"     // documenter
)

var DomainAgents = map[AgentDomain][]string{
    DomainCore:        {"coder", "researcher", "refactor", "architect"},
    DomainQuality:     {"tester", "reviewer", "debugger"},
    DomainSecurity:    {"security"},
    DomainPerformance: {"performance"},
    DomainSupport:     {"documenter"},
}
```

### 3.3 Agent Definition Format

Following Downshift's existing `mechanical-worker.md` format, extended with
Ruflo-inspired metadata:

```markdown
---
name: coder
description: Implementation specialist focused on writing clean, tested code
domain: core
mode: write
default_tier: mid
capabilities:
  - implementation
  - tdd
  - code-quality
  - bug-fix
priority: 1
tools: Read, Grep, Glob, Edit, Bash
---

# Coder Agent

You implement features and fix bugs with clean, tested code.

## Rules

1. **Read before write.** Understand the codebase before changing it.
2. **TDD when appropriate.** Write test first for new behavior.
3. **Minimal change.** Smallest diff that solves the problem.
4. **No scope creep.** Stay inside the assigned files.
5. **Report blockers.** If you need a decision, stop and say so.

## Verification

Run `downshift compactor -- <test command>` before reporting done.

## Output Format

Report: file changed, lines modified, test command run, exit status.
```

---

## 4. Memory Coordination Protocol

### 4.1 State File Structure

Location: `.downshift/swarm/state.json`

```json
{
  "swarm_id": "swarm-20261102-abc123",
  "created_at": "2026-11-02T10:00:00Z",
  "status": "running",
  "topology": "star",
  "coordinator": "orchestrator-session",
  "workers": {
    "worker-001": {
      "agent": "coder",
      "domain": "core",
      "status": "active",
      "task": "Implement login endpoint",
      "started_at": "2026-11-02T10:01:00Z",
      "progress": "Writing tests",
      "last_heartbeat": "2026-11-02T10:05:30Z"
    },
    "worker-002": {
      "agent": "tester",
      "domain": "quality",
      "status": "idle",
      "task": null,
      "started_at": null,
      "progress": null,
      "last_heartbeat": "2026-11-02T10:05:28Z"
    }
  },
  "tasks": {
    "task-001": {
      "title": "Implement login endpoint",
      "status": "in-progress",
      "assigned_to": "worker-001",
      "domain": "core",
      "dependencies": [],
      "created_at": "2026-11-02T10:00:30Z"
    }
  },
  "shared": {
    "decisions": [
      {
        "id": "dec-001",
        "question": "Use JWT or session cookies?",
        "answer": "JWT",
        "decided_by": "architect",
        "timestamp": "2026-11-02T10:02:00Z"
      }
    ]
  }
}
```

### 4.2 Worker Status Protocol

Workers update their status via a simple file write (the orchestrator skill
instructs them to do this):

```bash
# Worker writes its status (via the prompt instructions)
# The coordinator polls the file
```

Since workers are isolated and can't access the Downshift MCP, the coordination
happens through the parent:
1. Parent writes initial state
2. Parent includes "report status to collect()" in worker prompt
3. Parent polls collect() and updates state file
4. Parent reads state file to coordinate

This is a **simulation** of Ruflo's memory MCP, adapted to Downshift's
isolation model.

---

## 5. Topology Selection

### 5.1 Topology Types

```go
// internal/orchestration/topology.go

type TopologyType string

const (
    TopologyStar         TopologyType = "star"         // One coordinator, N workers
    TopologyMesh         TopologyType = "mesh"         // Workers can see each other's state
    TopologyHierarchical TopologyType = "hierarchical" // Domain leads coordinate sub-workers
)

func SelectTopology(task TaskSpec) TopologyType {
    // Star: simple tasks, single domain
    if len(task.Domains) == 1 && task.Complexity < 5 {
        return TopologyStar
    }
    
    // Hierarchical: multi-domain with dependencies
    if len(task.Domains) > 2 || len(task.Dependencies) > 0 {
        return TopologyHierarchical
    }
    
    // Mesh: parallel work across domains, no dependencies
    return TopologyMesh
}
```

### 5.2 Topology Diagrams

**Star (simple):**
```
            Coordinator
           /    |    \
        Coder  Tester  Reviewer
```

**Mesh (parallel):**
```
        Coordinator
       /     |     \
    Coder--Tester--Reviewer
       \     |     /
        (shared state)
```

**Hierarchical (complex):**
```
              Coordinator
             /     |     \
      Core Lead  Quality Lead  Security Lead
        /   \        |            |
    Coder  Refactor  Tester     Security
              |
          Researcher
```

---

## 6. New MCP Tools

### 6.1 Proposed Tools

| Tool | Description | Schema |
|------|-------------|--------|
| `swarm_init` | Initialize a new swarm with topology | `{topology, agents[], task}` |
| `swarm_status` | Get current swarm state | `{}` |
| `swarm_assign` | Assign task to domain/agent | `{task_id, domain, agent?}` |
| `swarm_coordinate` | Send coordination message | `{from, to, type, payload}` |
| `swarm_shutdown` | Gracefully stop swarm | `{swarm_id}` |

### 6.2 Integration with Existing Tools

The new tools **wrap** existing tools, they don't replace them:

```go
// swarm_init internally calls:
// 1. Creates state file
// 2. Calls delegate() for each agent
// 3. Returns swarm_id

// swarm_status internally calls:
// 1. Reads state file
// 2. Calls collect() to update worker status
// 3. Returns aggregated state
```

---

## 7. Implementation Roadmap

### Phase 1: Agent Definitions (1-2 days)
- [ ] Create 10 agent definition files in `agents/`
- [ ] Follow existing `mechanical-worker.md` format
- [ ] Add domain and capability metadata
- [ ] Update `skills/README.md` with agent catalog

### Phase 2: Memory Layer (2-3 days)
- [ ] Create `internal/orchestration/memory.go`
- [ ] Implement state file read/write
- [ ] Add file locking for concurrent access
- [ ] Write tests for state transitions

### Phase 3: Coordinator Logic (3-4 days)
- [ ] Create `internal/orchestration/coordinator.go`
- [ ] Implement topology selection
- [ ] Implement domain routing
- [ ] Integrate with existing delegate/collect

### Phase 4: MCP Tools (2-3 days)
- [ ] Add swarm_init tool
- [ ] Add swarm_status tool
- [ ] Add swarm_assign tool
- [ ] Add swarm_coordinate tool
- [ ] Add swarm_shutdown tool

### Phase 5: Skills & Documentation (2 days)
- [ ] Create `skills/downshift-swarm/SKILL.md`
- [ ] Update orchestrator skill with swarm support
- [ ] Write user guide
- [ ] Add examples

### Total Estimate: 10-14 days

---

## 8. Risk Analysis

### 8.1 Technical Risks

| Risk | Mitigation |
|------|------------|
| File locking on state.json | Use flock() on Unix, LockFileEx on Windows |
| Worker can't write to shared state | Parent proxies all state updates |
| Topology complexity | Start with star only, add others later |
| Breaking existing behavior | All changes are additive, existing tools unchanged |

### 8.2 Scope Risks

| Risk | Mitigation |
|------|------------|
| Feature creep | Strict phase gates, PR per phase |
| Over-engineering | Start minimal, iterate based on usage |
| Ruflo divergence | Document what we took and what we changed |

---

## 9. Success Criteria

### 9.1 Phase 1 Done When
- [ ] All 10 agents defined
- [ ] Each agent has tests for prompt generation
- [ ] Agents are loadable by orchestrator skill

### 9.2 Phase 2 Done When
- [ ] State file survives gateway restart
- [ ] Multiple workers can be tracked
- [ ] Status transitions are atomic

### 9.3 Phase 3 Done When
- [ ] Coordinator can run star topology
- [ ] Domain routing works for all 5 domains
- [ ] Parallel workers coordinate via state

### 9.4 MVP Done When
- [ ] User can say "swarm: implement X"
- [ ] Coordinator selects agents automatically
- [ ] Results are collected and integrated
- [ ] Cost is tracked per worker

---

## 10. Appendix: Ruflo REA Findings

### 10.1 Components Extracted

| Ruflo Component | Downshift Adaptation |
|-----------------|---------------------|
| `UnifiedSwarmCoordinator` | `internal/orchestration/coordinator.go` |
| `AgentPool` | Part of coordinator (simpler) |
| `MessageBus` | State file + collect() polling |
| `ConsensusEngine` | Not needed (parent decides) |
| `TopologyManager` | `internal/orchestration/topology.go` |
| `DomainConfig` | `internal/orchestration/domain.go` |

### 10.2 Components NOT Adopted

| Ruflo Component | Why Not |
|-----------------|---------|
| Consensus (Raft/Byzantine) | Overkill for parent-controlled swarm |
| Memory MCP | Workers are isolated; use file instead |
| Worker-to-worker messaging | Violates isolation model |
| Agent hierarchy (workers spawn workers) | Deliberate design choice |

### 10.3 Key Learnings

1. **Domain organization is powerful.** Grouping agents by capability simplifies routing.
2. **State file is enough.** Don't need a database for coordination.
3. **Flat is OK.** Ruflo's hierarchy is for 15+ agents; 10 agents work fine flat.
4. **Isolation is a feature.** Keep workers dumb; coordinator is smart.

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift-ai
