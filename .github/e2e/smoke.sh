#!/usr/bin/env bash
# Drives a real yomihon server against the fixture vault next to this script and
# asserts that each reading face renders, then asserts the listening socket is
# bound to loopback and to nothing else. The socket assertion is the live form
# of the promise that the server is reachable only from this machine: if the
# bind address ever widens to 0.0.0.0 or ::, this turns red. Runs on Linux (ss)
# and macOS (lsof) so it can be exercised locally as well as on the runner.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bin="${1:-${YOMIHON_BIN:-}}"
if [ -z "$bin" ]; then
  echo "usage: smoke.sh <path-to-yomihon-binary>   (or set YOMIHON_BIN)" >&2
  exit 2
fi
vault="$here/vault"
port="${YOMIHON_SMOKE_PORT:-19733}"
base="http://127.0.0.1:${port}"
log="$(mktemp)"
body="$(mktemp)"

YOMIHON_ROOT="$vault" YOMIHON_PORT="$port" "$bin" serve >"$log" 2>&1 &
server_pid=$!
cleanup() {
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
  rm -f "$log" "$body"
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; [ -s "$log" ] && { echo "--- server log ---" >&2; cat "$log" >&2; }; exit 1; }

# Readiness: poll a face that needs only the running server until it answers.
ready=""
for _ in $(seq 1 60); do
  if curl -fsS -o /dev/null "${base}/search" 2>/dev/null; then ready=1; break; fi
  sleep 0.25
done
[ -n "$ready" ] || fail "server never became ready on ${base}"

# Each face: 200 after following redirects, plus a marker that proves the right
# page rendered rather than a blank 200.
assert_face() {
  local path="$1" marker="$2" code
  code="$(curl -fsSL -o "$body" -w '%{http_code}' "${base}${path}")" || fail "GET ${path} did not return success"
  [ "$code" = "200" ] || fail "GET ${path} status ${code}, want 200"
  grep -qF "$marker" "$body" || fail "GET ${path} rendered but is missing its marker: ${marker}"
  echo "ok: ${path}"
}

# Home redirects to the reading page for the vault's README.
loc="$(curl -fsS -o /dev/null -w '%{redirect_url}' "${base}/")"
case "$loc" in
  */notes/README.md) echo "ok: / -> ${loc}" ;;
  *) fail "/ redirected to '${loc}', want the README reading page" ;;
esac

assert_face "/"                       "<title>README"
assert_face "/notes/Notes/alpha.md"   "tortoise"
assert_face "/syllabus/Maps/study.md" "<title>Study Path"
assert_face "/search?q=tortoise"      'href="/notes/Notes/alpha.md"'

# The reachability promise, checked on the live socket.
echo "checking the listening socket is loopback-only..."
if command -v ss >/dev/null 2>&1; then
  engine="ss"
  sockets="$(ss -ltnH 2>/dev/null | grep -E "[:.]${port}([^0-9]|$)" || true)"
elif command -v lsof >/dev/null 2>&1; then
  engine="lsof"
  sockets="$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true)"
else
  fail "neither ss nor lsof is available to inspect the listening socket"
fi
[ -n "$sockets" ] || fail "no listening socket found on port ${port}"
echo "${sockets}"
printf '%s\n' "$sockets" | grep -qE "127\.0\.0\.1[:.]${port}" || fail "socket is not bound to 127.0.0.1 (${engine})"
if printf '%s\n' "$sockets" | grep -qE "(0\.0\.0\.0|\[::\]|\*)[:.]${port}([^0-9]|$)"; then
  fail "socket is bound to a non-loopback address (${engine}); it would be reachable off this machine"
fi
echo "ok: listening only on 127.0.0.1:${port}"

echo "smoke passed"
