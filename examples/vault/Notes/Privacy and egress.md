---
title: Privacy and egress
type: note
status: ready
created: 2026-02-16
lang: en
---

yomihon makes no network call. Not a gated one, not an opt-in one: it opens
your files and a loopback socket, and it holds no client and no credential with
which to do anything else.

That is a shorter promise than it used to be. There were three named exceptions,
all of them for a semantic search face that sent note content and query text to
an embedding provider. That face was removed once it was clear nobody was using
it, and the exceptions went with it — a rule with no exceptions is one you can
check rather than one you have to trust.

`[privacy] never_egress_dirs` still means something: it names directories an
agent's output may not draw from. It is empty in this vault because there is
nothing here to withhold.
