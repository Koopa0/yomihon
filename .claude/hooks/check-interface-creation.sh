#!/bin/bash
# PostToolUse hook: when a .go file is written/edited, check for new interface definitions
# Advisory (exit 0) — reminds Claude to justify interfaces

source "$(dirname "$0")/parse-hook-input.sh"

input=$(cat)
FILE_PATH=$(parse_file_path "$input")

# Only check .go files, skip test files and generated code
[[ "$FILE_PATH" != *.go ]] && exit 0
[[ "$FILE_PATH" == *_test.go ]] && exit 0
[[ "$FILE_PATH" == */db/* ]] && exit 0

# Check if file contains interface definitions.
# Matches exported/unexported, grouped type blocks, and no-space braces;
# 'interface{}' (empty interface) is excluded.
MATCHES=$(grep -E '^[[:space:]]*(type[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*[[:space:]]+interface[[:space:]]*\{' "$FILE_PATH" 2>/dev/null | grep -vE 'interface[[:space:]]*\{\}')
if [[ -n "$MATCHES" ]]; then
  IFACE_NAMES=$(printf '%s\n' "$MATCHES" | sed -E 's/^[[:space:]]*(type[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*).*/\2/' | tr '\n' ',' | sed 's/,$//')
  echo "INTERFACE CHECK: $FILE_PATH defines interface(s): $IFACE_NAMES." >&2
  echo "Interfaces are DISCOVERED, not designed. For EACH interface: (1) Is this a consumer-boundary subset of an existing concrete type in another feature? That is the legitimate discovery case." >&2
  echo "(2) Otherwise: how many production implementations? If only 1 → use the concrete type." >&2
  echo "(3) If deleted, what production code breaks? If only tests → use testcontainers." >&2
  echo "See rules/interfaces.md and rules/go-philosophy.md § Semantic Design." >&2
fi

exit 0
