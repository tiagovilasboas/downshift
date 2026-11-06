# Agent Framework Diagrams

Visual diagrams for the Downshift Agent Framework spike.

---

## 1. Architecture Overview

```mermaid
graph TB
    subgraph "Layer 4: Orchestration Skills"
        COORD[coordinator.md]
        SWARM[swarm-init.md]
        TOPO[topology.md]
    end

    subgraph "Layer 3: Agent Definitions"
        subgraph "Core Domain"
            CODER[coder]
            RESEARCH[researcher]
            REFACTOR[refactor]
            ARCH[architect]
        end
        subgraph "Quality Domain"
            TESTER[tester]
            REVIEW[reviewer]
            DEBUG[debugger]
        end
        subgraph "Other Domains"
            SEC[security]
            PERF[performance]
            DOC[documenter]
        end
    end

    subgraph "Layer 2: Coordination Engine"
        COORDGO[coordinator.go]
        MEMGO[memory.go]
        TOPOGO[topology.go]
        REGGO[registry.go]
        DOMGO[domain.go]
    end

    subgraph "Layer 1: Downshift Core"
        DEL[delegate]
        COL[collect]
        DIF[diff]
        SEND[send]
        STEER[steer]
    end

    COORD --> COORDGO
    SWARM --> TOPOGO
    TOPO --> TOPOGO

    COORDGO --> MEMGO
    COORDGO --> REGGO
    COORDGO --> DOMGO
    TOPOGO --> COORDGO

    REGGO --> CODER
    REGGO --> TESTER
    REGGO --> REVIEW
    REGGO --> DEBUG
    REGGO --> RESEARCH
    REGGO --> REFACTOR
    REGGO --> ARCH
    REGGO --> SEC
    REGGO --> PERF
    REGGO --> DOC

    COORDGO --> DEL
    COORDGO --> COL
    COORDGO --> DIF
```

---

## 2. Data Flow

```mermaid
sequenceDiagram
    participant U as User
    participant O as Orchestrator
    participant C as Coordinator
    participant M as Memory (state.json)
    participant D as Downshift MCP
    participant W as Worker Process

    U->>O: "Implement login with tests"
    O->>C: Parse requirements
    C->>C: Select topology (star)
    C->>C: Route to domains (core, quality)
    
    C->>M: Write initial state
    Note over M: swarm_id, status: running

    C->>D: delegate(agent=coder, task=...)
    D->>W: Spawn Claude CLI
    C->>D: delegate(agent=tester, task=...)
    D->>W: Spawn Claude CLI

    loop Poll for results
        C->>D: collect()
        D-->>C: Worker results
        C->>M: Update worker status
    end

    W-->>D: Task complete
    D-->>C: Result via collect()
    C->>M: Mark task completed
    C-->>O: Aggregated results
    O-->>U: Implementation + tests done
```

---

## 3. Topology Types

### Star Topology (Simple Tasks)

```mermaid
graph TB
    COORD((Coordinator))
    W1[Coder]
    W2[Tester]
    W3[Reviewer]

    COORD --> W1
    COORD --> W2
    COORD --> W3
```

### Mesh Topology (Parallel Work)

```mermaid
graph TB
    COORD((Coordinator))
    W1[Coder]
    W2[Tester]
    W3[Reviewer]
    STATE[(Shared State)]

    COORD --> W1
    COORD --> W2
    COORD --> W3
    
    W1 -.-> STATE
    W2 -.-> STATE
    W3 -.-> STATE
    
    STATE -.-> W1
    STATE -.-> W2
    STATE -.-> W3
```

### Hierarchical Topology (Complex Multi-Domain)

```mermaid
graph TB
    COORD((Coordinator))
    
    subgraph Core Domain
        CL[Core Lead]
        CODER[Coder]
        REFACTOR[Refactor]
    end
    
    subgraph Quality Domain
        QL[Quality Lead]
        TESTER[Tester]
        DEBUG[Debugger]
    end
    
    subgraph Security Domain
        SL[Security Lead]
        SEC[Security]
    end

    COORD --> CL
    COORD --> QL
    COORD --> SL
    
    CL --> CODER
    CL --> REFACTOR
    
    QL --> TESTER
    QL --> DEBUG
    
    SL --> SEC
```

---

## 4. State Machine: Worker Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Pending: Task created
    Pending --> Queued: Task assigned to domain
    Queued --> Assigned: Worker selected
    Assigned --> InProgress: Worker starts
    InProgress --> Completed: Success
    InProgress --> Failed: Error
    InProgress --> Blocked: Needs decision
    Blocked --> InProgress: Decision provided
    Failed --> Queued: Retry (if allowed)
    Completed --> [*]
    Failed --> [*]: Max retries
```

---

## 5. Domain Routing

```mermaid
flowchart LR
    TASK[Task] --> ROUTER{Domain Router}
    
    ROUTER -->|implementation| CORE[Core Domain]
    ROUTER -->|testing| QUALITY[Quality Domain]
    ROUTER -->|audit| SEC[Security Domain]
    ROUTER -->|benchmark| PERF[Performance Domain]
    ROUTER -->|docs| SUPPORT[Support Domain]
    
    CORE --> CODER[coder]
    CORE --> RESEARCH[researcher]
    CORE --> REFACTOR[refactor]
    CORE --> ARCH[architect]
    
    QUALITY --> TESTER[tester]
    QUALITY --> REVIEW[reviewer]
    QUALITY --> DEBUG[debugger]
    
    SEC --> SECURITY[security]
    
    PERF --> PERFORMANCE[performance]
    
    SUPPORT --> DOC[documenter]
```

---

## 6. Memory File Structure

```mermaid
classDiagram
    class SwarmState {
        +string swarm_id
        +datetime created_at
        +SwarmStatus status
        +TopologyType topology
        +string coordinator
        +Map~string,WorkerState~ workers
        +Map~string,TaskState~ tasks
        +SharedState shared
    }
    
    class WorkerState {
        +string agent
        +AgentDomain domain
        +WorkerStatus status
        +string task
        +datetime started_at
        +string progress
        +datetime last_heartbeat
    }
    
    class TaskState {
        +string title
        +TaskStatus status
        +string assigned_to
        +AgentDomain domain
        +string[] dependencies
        +datetime created_at
    }
    
    class SharedState {
        +Decision[] decisions
    }
    
    class Decision {
        +string id
        +string question
        +string answer
        +string decided_by
        +datetime timestamp
    }
    
    SwarmState "1" --> "*" WorkerState
    SwarmState "1" --> "*" TaskState
    SwarmState "1" --> "1" SharedState
    SharedState "1" --> "*" Decision
```

---

## 7. Integration with Existing Downshift

```mermaid
graph LR
    subgraph "New (internal/orchestration)"
        COORD[coordinator.go]
        MEM[memory.go]
        TOPO[topology.go]
        REG[registry.go]
    end
    
    subgraph "Existing (internal/workers)"
        QUEUE[queue.go]
        LAUNCH[launch_agents.go]
        DIFF[diff.go]
        STEER[steer.go]
    end
    
    subgraph "Existing (internal/mcp/tools)"
        DELEGATE[delegate]
        COLLECT[collect]
        DIFFMCP[diff]
        SENDMCP[send]
    end
    
    COORD --> DELEGATE
    COORD --> COLLECT
    COORD --> DIFFMCP
    
    DELEGATE --> QUEUE
    QUEUE --> LAUNCH
    
    MEM -.->|reads/writes| STATE[(state.json)]
    
    style COORD fill:#90EE90
    style MEM fill:#90EE90
    style TOPO fill:#90EE90
    style REG fill:#90EE90
```

---

## 8. Roadmap Timeline

```mermaid
gantt
    title Agent Framework Implementation
    dateFormat  YYYY-MM-DD
    
    section Phase 1
    Agent Definitions (10)     :a1, 2026-11-03, 2d
    
    section Phase 2
    Memory Layer              :a2, after a1, 3d
    
    section Phase 3
    Coordinator Logic         :a3, after a2, 4d
    
    section Phase 4
    MCP Tools                 :a4, after a3, 3d
    
    section Phase 5
    Skills & Docs             :a5, after a4, 2d
    
    section Milestones
    MVP Complete              :milestone, m1, after a5, 0d
```

---

Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift
