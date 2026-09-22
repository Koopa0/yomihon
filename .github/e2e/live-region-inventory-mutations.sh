#!/usr/bin/env bash
# Source mutations in disposable copies: no server, browser or generated files.
# Only this inventory test runs; a compile error cannot count as a caught edit.
set -euo pipefail

export GOMAXPROCS=2
export GOFLAGS=-p=2

repo=$(git rev-parse --show-toplevel)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/yomihon-live-inventory.XXXXXX")
trap 'rm -rf "$scratch"' EXIT

git -C "$repo" archive --output="$scratch/source.tar" HEAD
mkdir "$scratch/baseline"
tar -xf "$scratch/source.tar" -C "$scratch/baseline"
(
  cd "$scratch/baseline"
  go test ./internal/archlock -run '^TestLiveRegionOwnershipInventory$' -count=1
)

for mutation in added-template added-js removed-role removed-live moved-role; do
  copy="$scratch/$mutation"
  mkdir "$copy"
  tar -xf "$scratch/source.tar" -C "$copy"
  python3 - "$copy" "$mutation" <<'PY'
from pathlib import Path
import sys

root = Path(sys.argv[1])
mode = sys.argv[2]


def replace_once(path, old, new):
    source = path.read_text()
    count = source.count(old)
    if count != 1:
        raise SystemExit(f"NOT-APPLIED {mode}: {path.name} needle matched {count}, want 1")
    path.write_text(source.replace(old, new, 1))


if mode == "added-template":
    path = root / "internal/ui/layouts/live_region_inventory_probe.templ"
    if path.exists():
        raise SystemExit(f"NOT-APPLIED {mode}: probe path already exists")
    path.write_text('package layouts\n\ntempl LiveRegionInventoryProbe() {\n'
                    '\t<p class="y-inventory-probe" role="status"></p>\n}\n')
elif mode == "added-js":
    path = root / "assets/js/live-region-inventory-probe.js"
    if path.exists():
        raise SystemExit(f"NOT-APPLIED {mode}: probe path already exists")
    path.write_text("const inventoryProbe = document.createElement('p');\n"
                    "inventoryProbe.className = 'y-inventory-probe';\n"
                    "inventoryProbe.setAttribute('aria-live', 'polite');\n")
elif mode == "removed-role":
    replace_once(root / "internal/ui/layouts/reply.templ", ' role="status"', '')
elif mode == "removed-live":
    replace_once(root / "assets/js/lesson.js", "    speechStatus.setAttribute('aria-live', 'polite');\n", '')
elif mode == "moved-role":
    path = root / "internal/ui/pages/slotmachine.templ"
    replace_once(path, '<p class="y-slotlive y-offscreen" role="status"', '<p class="y-slotlive y-offscreen"')
    replace_once(path, '<p class="y-slotgloss">', '<p class="y-slotgloss" role="status">')
else:
    raise SystemExit(f"unknown mutation: {mode}")
PY

  log="$scratch/$mutation.log"
  code=0
  (
    cd "$copy"
    go test ./internal/archlock -run '^TestLiveRegionOwnershipInventory$' -count=1
  ) >"$log" 2>&1 || code=$?
  cat "$log"
  if [[ "$code" != 1 ]] || ! grep -Fq -- '--- FAIL: TestLiveRegionOwnershipInventory (' "$log"; then
    echo "FAIL live-region-inventory: $mutation exited $code without the expected test failure" >&2
    exit 1
  fi

  case "$mutation" in
    added-template)
      expected='added live-region declaration: internal/ui/layouts/live_region_inventory_probe.templ LiveRegionInventoryProbe/p.y-inventory-probe role=status'
      ;;
    added-js)
      expected='added live-region declaration: assets/js/live-region-inventory-probe.js inventoryProbe/p.y-inventory-probe aria-live=polite'
      ;;
    removed-role)
      expected='missing live-region declaration: internal/ui/layouts/reply.templ Reply/p.y-reply role=status'
      ;;
    removed-live)
      expected='missing live-region declaration: assets/js/lesson.js speechStatus/span.y-ttsbar__status aria-live=polite'
      ;;
    moved-role)
      expected='missing live-region declaration: internal/ui/pages/slotmachine.templ slotCard/p.y-slotlive.y-offscreen role=status'
      grep -Fq -- 'added live-region declaration: internal/ui/pages/slotmachine.templ slotCard/p.y-slotgloss role=status' "$log" || {
        echo 'FAIL live-region-inventory: moved declaration did not identify its new element' >&2
        exit 1
      }
      ;;
  esac
  if ! grep -Fq -- "$expected" "$log"; then
    echo "FAIL live-region-inventory: $mutation failed without identifying the changed declaration" >&2
    exit 1
  fi
  echo "MUTATE-RESULT: caught $mutation"
done
