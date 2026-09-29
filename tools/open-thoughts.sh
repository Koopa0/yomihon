#!/usr/bin/env bash
# Every browser write is confined to the canonical harness's disposable copy.
set -euo pipefail
if [ "${CI:-}" != true ] || [ "$#" -ne 1 ]; then
  echo 'usage in CI: bash tools/open-thoughts.sh <built-yomihon>' >&2
  exit 2
fi
work="$(mktemp -d "${TMPDIR:-/tmp}/yomihon-open-thoughts.XXXXXX")"
trap 'rm -rf "$work"' EXIT
node tools/open-thoughts.mjs --fixture "$work/vault"
YOMIHON_FIXTURE="$work/vault" bash .github/e2e/serve.sh "$1" 19772 -- node tools/open-thoughts.mjs
