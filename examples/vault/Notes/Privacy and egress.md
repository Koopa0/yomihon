---
title: Privacy and egress
type: note
status: ready
created: 2026-02-16
lang: en
---

yomihon makes no network call. It opens your files and a loopback socket, and holds no client and no credential with which to do anything else.

`[privacy] never_egress_dirs` in the contract names directories an agent's output may not draw from. It is empty in this vault because there is nothing here to withhold.
