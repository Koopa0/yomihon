#!/usr/bin/env bash
# Asserts that each reading face renders on an already-running yomihon server,
# then asserts the listening socket is bound to loopback and to nothing else.
# The socket assertion is the live form of the promise that the server is
# reachable only from this machine: if the bind address ever widens to 0.0.0.0
# or ::, this turns red. Runs on Linux (ss) and macOS (lsof) so it can be
# exercised locally as well as on the runner.
#
# serve.sh starts the server and exports the two variables this reads:
#
#   bash .github/e2e/serve.sh ./bin/yomihon 19733 -- bash .github/e2e/smoke.sh
#
# The socket assertion can never go red against a real server, because a real
# server binds loopback. So it carries its own proof that it can:
#
#   bash .github/e2e/smoke.sh --self-test
#
# drives the same verdict over recorded listings from both engines — a widened
# bind, a wildcard bind, an address family added alongside loopback — and
# requires each to be refused. It needs no server, and it runs in CI beside the
# live pass.
set -euo pipefail

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# Reads a listing of listening sockets and reports whether it describes a socket
# bound to loopback and to nothing else. Empty means the server is not listening
# at all, which is not the promise either. Echoing the reason lets one caller
# print it and the self-test compare against it.
loopback_only() {
  local listing="$1" port="$2"
  if [ -z "$listing" ]; then
    echo "no listening socket found on port ${port}"
    return 1
  fi
  if ! printf '%s\n' "$listing" | grep -qE "127\.0\.0\.1[:.]${port}"; then
    echo "socket is not bound to 127.0.0.1"
    return 1
  fi
  if printf '%s\n' "$listing" | grep -qE "(0\.0\.0\.0|\[::\]|\*)[:.]${port}([^0-9]|$)"; then
    echo "socket is bound to a non-loopback address; it would be reachable off this machine"
    return 1
  fi
  return 0
}

# The verdict above is the live form of a wall, and against a real server it can
# only ever come back green. These recordings are what make it a lock: each is a
# listing a real engine would print, and each is answered for by exactly one of
# the verdict's three refusals, which is why the expected reason is compared and
# not merely the accept-or-refuse. A listing refused for the wrong reason has
# found a hole in a different check than the one it was written to hold. Remove
# any one refusal — the empty listing, the missing loopback address, any single
# address family in the widened-bind pattern — and a row below turns red while
# this file's live pass, against a server that binds loopback, goes on passing.
self_test() {
  local port=19733 failures=0
  check() { # <name> <want: accepted|the reason> <listing>
    local name="$1" want="$2" listing="$3" reason status
    reason="$(loopback_only "$listing" "$port")" && status=accepted || status=refused
    if [ "$status" = accepted ]; then
      reason=accepted
    fi
    case "$reason" in
    *"$want"*)
      echo "  ok: ${name} -> ${reason}"
      ;;
    *)
      echo "  SELF-TEST FAIL: ${name} -> ${reason}, want ${want}" >&2
      failures=1
      ;;
    esac
  }

  check "lsof, loopback only" \
    "accepted" \
    "yomihon 1 koopa 6u IPv4 0x1 0t0 TCP 127.0.0.1:19733 (LISTEN)"
  check "ss, loopback only" \
    "accepted" \
    "LISTEN 0 4096 127.0.0.1:19733 0.0.0.0:*"
  check "nothing listening" \
    "no listening socket found" \
    ""
  check "lsof, bound to a machine-local network address" \
    "not bound to 127.0.0.1" \
    "yomihon 1 koopa 6u IPv4 0x1 0t0 TCP 192.168.1.5:19733 (LISTEN)"
  check "lsof, loopback and 0.0.0.0 together" \
    "non-loopback address" \
    "yomihon 1 koopa 6u IPv4 0x1 0t0 TCP 127.0.0.1:19733 (LISTEN)
yomihon 1 koopa 7u IPv4 0x2 0t0 TCP 0.0.0.0:19733 (LISTEN)"
  check "ss, loopback and every v6 address together" \
    "non-loopback address" \
    "LISTEN 0 4096 127.0.0.1:19733 0.0.0.0:*
LISTEN 0 4096 [::]:19733 [::]:*"
  check "lsof, loopback and a wildcard bind together" \
    "non-loopback address" \
    "yomihon 1 koopa 6u IPv4 0x1 0t0 TCP 127.0.0.1:19733 (LISTEN)
yomihon 1 koopa 7u IPv4 0x2 0t0 TCP *:19733 (LISTEN)"

  [ "$failures" -eq 0 ] || fail "the loopback socket verdict no longer refuses a widened bind"
  echo "self-test passed: every widened bind it was shown was refused, each for its own reason"
}

if [ "${1:-}" = "--self-test" ]; then
  self_test
  exit 0
fi

base="${YOMIHON_BASE:?smoke.sh needs a running server; start it with serve.sh}"
port="${YOMIHON_PORT:?smoke.sh needs a running server; start it with serve.sh}"
body="$(mktemp)"
trap 'rm -f "$body"' EXIT

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

assert_face "/" "<title>README"
assert_face "/notes/Notes/alpha.md" "tortoise"
assert_face "/syllabus/Maps/study.md" "<title>Study Path"
assert_face "/search?q=tortoise" 'href="/notes/Notes/alpha.md"'

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
if [ -n "$sockets" ]; then
  echo "${sockets}"
fi
if ! reason="$(loopback_only "$sockets" "$port")"; then
  fail "${reason} (${engine})"
fi
echo "ok: listening only on 127.0.0.1:${port}"

echo "smoke passed"
