#!/usr/bin/env bash
# Idempotent Cloud Agent bootstrap for yomihon. Prepares the toolchain and
# generated state a fresh checkout needs to build and serve the reader:
#
#   - the Go module cache (which also fetches the go.mod templ tool),
#   - the Node dev tools the e2e browser probes drive,
#   - the committed generated sources (templ output), and
#   - a compiled server binary so the terminal can serve immediately.
#
# It touches nothing under version control except regenerating already-committed
# generated files, so a second run converges without changes.
set -euo pipefail

# Populate the module cache and materialise the go.mod tool (templ) so the
# build and generation steps below run offline-fast and deterministically.
go mod download
go tool templ --version >/dev/null

# Frontend linters and the Playwright driver the e2e browser probes use. The
# product build needs none of this; it lives only under .github/.
npm ci --prefix .github --ignore-scripts --no-audit --fund=false

# Regenerate the committed generated sources the Go build depends on. A clean
# tree leaves these unchanged; a stale tree is repaired before the build.
go tool templ generate -path internal/ui

# Compile the server so a booted environment can serve without a cold build.
go build -o bin/yomihon ./cmd/yomihon

echo "yomihon environment ready"
