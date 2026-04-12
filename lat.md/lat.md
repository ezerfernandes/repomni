This directory defines the high-level concepts, business logic, and architecture of this project using markdown. It is managed by [lat.md](https://github.com/ezerfernandes/lat.md) — a tool that anchors source code to these definitions.

- [[architecture]] — Package layering, state locations, key design decisions
- [[config]] — Global and per-repo configuration, workflow states, interactive wizard
- [[injection]] — Inject/eject mechanism, manifest tracking, git exclusion
- [[branching]] — Branch creation, PR/MR submission, cleanup lifecycle
- [[forge]] — GitHub/GitLab CLI abstraction and PR state mapping
- [[sync]] — Code sync (fetch/pull), state sync (PR/MR status), umbrella command
- [[session]] — Claude Code and Codex session discovery, parsing, and analysis
- [[cli]] — Cobra command tree, common patterns, exec diff, UI layer
