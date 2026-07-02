#!/bin/bash
# SessionStart hook — verify Go toolchain + inject routing reminders

go_ver=$(go version 2>/dev/null | awk '{print $3}')
if [ -n "$go_ver" ]; then
  echo "ENV: $go_ver" >&2
else
  echo "WARNING: go not found in PATH" >&2
fi

# golangci-lint must be v2 — a stale v1 binary earlier in PATH silently
# breaks 'make lint' against the v2 config (observed 2026-06).
if which golangci-lint >/dev/null 2>&1; then
  golangci-lint version 2>/dev/null | grep -q 'version 2\.' || \
    echo "WARNING: golangci-lint is not v2 ($(golangci-lint version 2>/dev/null | head -n1)) — make lint will fail. Check for a stale binary: which -a golangci-lint" >&2
else
  echo "WARNING: golangci-lint not found" >&2
fi
which sqlc >/dev/null 2>&1 || echo "WARNING: sqlc not found" >&2
which goimports >/dev/null 2>&1 || echo "WARNING: goimports not found" >&2
which benchstat >/dev/null 2>&1 || echo "WARNING: benchstat not found — make bench-compare unavailable. Install: go install golang.org/x/perf/cmd/benchstat@latest" >&2
which govulncheck >/dev/null 2>&1 || echo "WARNING: govulncheck not found — make vuln / verify-spec will fail. Install: go install golang.org/x/vuln/cmd/govulncheck@latest" >&2

# Idiom ceiling: rules/go-version.md mandates modern idioms, but go-spec is a
# PORTABLE harness — dropped into a consumer repo on an older toolchain, the
# "mandatory" 1.25/1.26 forms won't compile. Surface the actual ceiling from the
# go.mod directive so the mandated idioms are gated by what the target can build.
if [ -f go.mod ]; then
  gomod_ver=$(awk '/^go [0-9]/{print $2; exit}' go.mod)
  if [ -n "$gomod_ver" ]; then
    major=${gomod_ver%%.*}
    rest=${gomod_ver#*.}
    minor=${rest%%.*}
    if [ "$major" -eq 1 ] 2>/dev/null; then
      if [ "$minor" -lt 25 ] 2>/dev/null; then
        echo "GO IDIOM CEILING: go.mod is $gomod_ver. rules/go-version.md 'mandatory' idioms are version-GATED — at <1.25 do NOT use testing/synctest, sync.WaitGroup.Go (1.25), new(expr)/errors.AsType[T] (1.26); use the pre-1.25 forms. Check the 'Since' column before applying any modern idiom." >&2
      elif [ "$minor" -lt 26 ] 2>/dev/null; then
        echo "GO IDIOM CEILING: go.mod is $gomod_ver. The 1.26 idioms in rules/go-version.md are NOT available here — use errors.As (not errors.AsType[T]) and a pointer helper / &T{} (not new(expr) with initial value). 1.25 idioms (synctest, wg.Go) are fine." >&2
      fi
    fi
  fi
fi

cat >&2 <<'EOF'
SKILL ROUTING: Before writing code, read /using-go-spec to find applicable skills.

SEARCH STRATEGY: You have codebase-retrieval (Augment Context Engine) available.
Use it FIRST for discovery and understanding queries — it finds cross-file relationships
that grep misses. Use grep AFTER for exact symbol lookup and exhaustive verification.
Decision: "What/how/why does X work?" → codebase-retrieval. "Find all uses of X" → grep.

GO SYMBOLS: gopls is wired via .lsp.json. For Go symbol work — definitions,
references, call hierarchy, rename — prefer LSP navigation over grep (semantic, not
textual). Claude defaults to grep unless reminded; for "where is X defined / who calls X"
use the LSP tool.
EOF

exit 0
