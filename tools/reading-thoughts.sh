#!/usr/bin/env bash
# Reuse the canonical server harness so these writes never touch a real vault
# or the operator's reader state. The binary is built by the workflow first.
set -euo pipefail

if [ "${CI:-}" != true ] || [ "$#" -ne 1 ]; then
  echo 'usage in CI: bash tools/reading-thoughts.sh <built-yomihon>' >&2
  exit 2
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/yomihon-thoughts.XXXXXX")"
trap 'rm -rf "$work"' EXIT
node tools/reading-thoughts.mjs --fixture "$work/vault"
YOMIHON_FIXTURE="$work/vault" bash .github/e2e/serve.sh "$1" 19771 -- node tools/reading-thoughts.mjs
