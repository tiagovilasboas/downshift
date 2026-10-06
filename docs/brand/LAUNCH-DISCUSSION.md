# Launch — GitHub Discussion

**Published:** [Discussion #55 — Harness Downshift is now Downshift](https://github.com/tiagovilasboas/downshift/discussions/55) (Announcements). Pin in the Discussions UI if needed.

**Category:** Announcements  
**Title:** Harness Downshift is now Downshift

---

The project started as a **coding harness** router (Claude Code, Cursor, Codex hooks). The core is broader: **deterministic model routing** for AI workloads: classify task complexity, apply catalog policy, pick tier **without an LLM in the routing loop**.

- **Same CLI:** `downshift`
- **Repo:** https://github.com/tiagovilasboas/downshift (redirect from `harness-downshift`)
- **Licence:** Apache 2.0 — https://github.com/tiagovilasboas/downshift/blob/main/LICENSE
- **Release:** https://github.com/tiagovilasboas/downshift/releases/tag/v0.1.0-beta.6
- **State dir:** `~/.downshift` (legacy `~/.harness-downshift` supported) — `downshift doctor`
- **Harness** is an integration category, not the product name

**Docs:** [ARCHITECTURE](https://github.com/tiagovilasboas/downshift/blob/main/docs/ARCHITECTURE.md) · [WHEN-TO-USE](https://github.com/tiagovilasboas/downshift/blob/main/docs/WHEN-TO-USE.md) · [BETA-EXIT](https://github.com/tiagovilasboas/downshift/blob/main/docs/BETA-EXIT.md)

**Install:**

```bash
curl -fsSL https://raw.githubusercontent.com/tiagovilasboas/downshift/main/install.sh | sh -s v0.1.0-beta.6
```

Questions and misroutes: open an issue with the `misroute` label or use Q&A in Discussions.
