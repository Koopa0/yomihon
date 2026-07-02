#!/bin/bash
# PostToolUse hook: detect interface definitions in test files
# Advisory (exit 0) — warns about test-only interfaces

source "$(dirname "$0")/parse-hook-input.sh"

input=$(cat)
FILE_PATH=$(parse_file_path "$input")

[[ "$FILE_PATH" != *_test.go ]] && exit 0

# Matches exported/unexported, grouped type blocks, and no-space braces.
# 'interface{}' (empty interface / pre-1.18 any) is excluded — that is a
# type usage, not a definition.
MATCHES=$(grep -E '^[[:space:]]*(type[[:space:]]+)?[A-Za-z_][A-Za-z0-9_]*[[:space:]]+interface[[:space:]]*\{' "$FILE_PATH" 2>/dev/null | grep -vE 'interface[[:space:]]*\{\}')
if [[ -n "$MATCHES" ]]; then
  IFACE_NAMES=$(printf '%s\n' "$MATCHES" | sed -E 's/^[[:space:]]*(type[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*).*/\2/' | tr '\n' ',' | sed 's/,$//')
  echo "WARNING: Interface(s) defined in test file '$FILE_PATH': $IFACE_NAMES." >&2
  echo "Tests are never a reason to introduce an interface — use testcontainers with real implementations, or a hand-written fake implementing an EXISTING consumer interface for non-containerizable externals. See rules/interfaces.md and rules/testing.md § Test Doubles." >&2
fi

exit 0
