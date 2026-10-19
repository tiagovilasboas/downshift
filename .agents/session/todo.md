# Task List — Downshift

- [x] **Cache-Tax & Multi-Agent Ergonomics (PR #182):**
  - Brand logs with `[downshift:cache-tax]`.
  - Auto-bypass subagents, background jobs, and task notifications (`task-notification`, `peer`, `peer-send-message`, `coordinator`, `scheduled-trigger`).
  - Enable native context compressor by default in `downshift setup`.
- [x] **Purge MiniLM Naming (PR #183):**
  - Purge all residual MiniLM names across codebase, docs, and CLI.
  - Establish `DOWNSHIFT_SEMANTIC_BOOST` env var for canonical in-process FNV word-hash semantic boost.
- [x] **Auto-Discover Models on Setup (PR #184):**
  - Exclude experimental harnesses (Kiro/KiroCrew) from `discovery.Defaults()`.
  - Wire non-blocking model auto-discovery in `downshift setup`.
  - Guard discovery against test mocks, dry-run, and undo.
  - Remove non-hermetic local-only `cls_stability_test.go`.
- [x] **UX Mods Installation (PR #185):**
  - Install `savvy-progress` and `filetree` mods during `downshift setup` for Claude Code.
  - Safe opt-out with `--no-plugin`.
