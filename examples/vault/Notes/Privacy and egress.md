---
title: Privacy and egress
type: note
status: ready
created: 2026-02-16
lang: en
---

The server listens on `127.0.0.1` and has no remote mode. Reading, rendering,
search and diagnostics never leave the machine.

Three things can, and each is named: note content allowed by the contract's
privacy policy, sent to an embedding provider to build local search vectors; the
query text of a semantic search you explicitly asked for; and the fixed synthetic
probes a developer certification runs. Nothing else.

`[privacy] never_egress_dirs` is empty in this vault because there is nothing
here to withhold. Name a top-level directory and nothing under it reaches an
agent's output or a provider, whatever else the contract allows.
