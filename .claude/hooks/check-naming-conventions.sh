#!/bin/bash
# PostToolUse hook: check .go files for naming anti-patterns
# Advisory (exit 0) — warns but does not block
# Covers: B5 (XxxInterface), B8 (XxxImpl)

source "$(dirname "$0")/parse-hook-input.sh"

input=$(cat)
FILE_PATH=$(parse_file_path "$input")

# Only check .go files, skip generated code
[[ "$FILE_PATH" != *.go ]] && exit 0
[[ "$FILE_PATH" == */db/* ]] && exit 0

# Integration test convention (rules/testing.md): files carrying
# //go:build integration must be named integration_test.go, and
# integration_test.go must carry the tag (else it runs in 'make test').
# Advisory both ways.
if [[ "$FILE_PATH" == *_test.go && -f "$FILE_PATH" ]]; then
  base=$(basename "$FILE_PATH")
  if grep -qE '^//go:build .*integration' "$FILE_PATH" 2>/dev/null; then
    if [[ "$base" != "integration_test.go" ]]; then
      echo "NAMING: $FILE_PATH carries //go:build integration but is not named integration_test.go." >&2
      echo "Integration tests live in exactly one integration_test.go per feature package. See rules/testing.md § Integration Tests." >&2
    fi
  elif [[ "$base" == "integration_test.go" ]]; then
    echo "NAMING: $FILE_PATH lacks the //go:build integration tag — it will run in 'make test'." >&2
    echo "Add '//go:build integration' as the first line. See rules/testing.md § Integration Tests." >&2
  fi
  exit 0
fi

[[ ! -f "$FILE_PATH" ]] && exit 0

# B8: type names containing "Impl" (e.g., StoreImpl, cacheImpl)
if grep -qE '^type [A-Za-z]*[Ii]mpl[A-Za-z]* (struct|interface) ' "$FILE_PATH" 2>/dev/null; then
  NAMES=$(grep -oE '^type [A-Za-z]*[Ii]mpl[A-Za-z]* (struct|interface)' "$FILE_PATH" | awk '{print $2}' | tr '\n' ', ' | sed 's/,$//')
  echo "NAMING: $FILE_PATH uses 'Impl' in type name(s): $NAMES." >&2
  echo "Name types by what they ARE, not that they implement something: PostgresStore, not StoreImpl." >&2
  echo "See rules/naming.md § No \"Impl\" in Type Names." >&2
fi

# B5: interface names ending in "Interface" (e.g., OrderInterface)
if grep -qE '^type [A-Z][a-zA-Z]+Interface interface ' "$FILE_PATH" 2>/dev/null; then
  NAMES=$(grep -oE '^type [A-Z][a-zA-Z]+Interface interface' "$FILE_PATH" | awk '{print $2}' | tr '\n' ', ' | sed 's/,$//')
  echo "NAMING: $FILE_PATH uses 'Interface' suffix: $NAMES." >&2
  echo "Use -er suffix (Reader, Writer) or behavior noun (OrderReader), not XxxInterface." >&2
  echo "See rules/naming.md § Interface Names." >&2
fi

exit 0
