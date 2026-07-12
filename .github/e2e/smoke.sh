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
# The socket and Home assertions can never go red against a correct live server,
# so they carry their own recorded proofs that they can:
#
#   bash .github/e2e/smoke.sh --self-test
#
# drives the same verdict over recorded listings from both engines — a widened
# bind, a wildcard bind, an address family added alongside loopback — and
# requires each to be refused. It needs no server, and it runs in CI beside the
# live pass.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# One named site per dashboard block, plus one content marker that only the
# fixture vault's rendered README can supply. The live verdict and its self-test
# share this table, so neither can silently stop checking one block.
home_markers=(
  'recent|data-home-block="recent"'
  'lifecycle|data-home-block="lifecycle"'
  'study-paths|data-home-block="study-paths"'
  'search|data-home-block="search"'
  'vault-readme|Home, linking to'
  'sidebar-paths|data-sidebar-group="paths"'
  'sidebar-maps|data-sidebar-group="maps"'
  'sidebar-map-fixture|data-map-tree="Maps/reading.md"'
  'sidebar-journal|data-sidebar-group="journal"'
  'topbar-advanceable|data-advanceable-chip'
)

# This is a set comparison, not an order oracle: deleting a marker from the
# live table must not also delete its self-test by construction.
required_home_sites=(recent lifecycle study-paths search vault-readme sidebar-paths sidebar-maps sidebar-map-fixture sidebar-journal topbar-advanceable)

check_home_marker_table() {
  local actual required
  actual="$(printf '%s\n' "${home_markers[@]%%|*}" | sort)"
  required="$(printf '%s\n' "${required_home_sites[@]}" | sort)"
  [ "$actual" = "$required" ] || fail "Home marker sites differ from the required block and README set"
}

home_body_error() {
  local content="$1" entry site marker
  for entry in "${home_markers[@]}"; do
    site="${entry%%|*}"
    marker="${entry#*|}"
    if ! grep -qF "$marker" <<<"$content"; then
      echo "missing ${site} marker: ${marker}"
      return 1
    fi
  done
}

# Reads a listing of listening sockets and reports whether it describes a socket
# bound to loopback and to nothing else. Empty means the server is not listening
# at all, which is not the promise either. Echoing the reason lets one caller
# print it and the self-test compare against it.
#
# Both address tests end the port at a non-digit, because a port number is a
# prefix of longer ones: asked about 1973, a line for 19733 answers. Without that
# bound the verdict reads a socket it was not asked about — it would call a
# server on 192.168.1.5:1973 loopback-only on the strength of something else
# listening on 127.0.0.1:19733. The callers below happen to hand it one port's
# lines, which is their property and not this one's.
loopback_only() {
  local listing="$1" port="$2"
  if [ -z "$listing" ]; then
    echo "no listening socket found on port ${port}"
    return 1
  fi
  if ! printf '%s\n' "$listing" | grep -qE "127\.0\.0\.1[:.]${port}([^0-9]|$)"; then
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
  check_home_marker_table
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
  # A port number is a prefix of longer ones. These two hold the loopback test to
  # the port it was asked about: the first is a loopback socket on a different
  # port that merely starts with this one, and the second sets that decoy beside
  # a server reachable from the network on the port that was asked about.
  check "lsof, loopback on a longer port that starts with this one" \
    "not bound to 127.0.0.1" \
    "something 1 koopa 6u IPv4 0x1 0t0 TCP 127.0.0.1:197330 (LISTEN)"
  check "lsof, that decoy beside a network-reachable bind on the asked port" \
    "not bound to 127.0.0.1" \
    "something 1 koopa 6u IPv4 0x1 0t0 TCP 127.0.0.1:197330 (LISTEN)
yomihon   2 koopa 7u IPv4 0x2 0t0 TCP 192.168.1.5:19733 (LISTEN)"

  # Build one complete Home recording from the same marker table the live
  # verdict reads. Then remove each named invariant in turn. This proves a blank
  # 200, a dashboard missing any block, and a dashboard missing only the README
  # content all go red for the invariant that was removed, regardless of table
  # order.
  local complete="" entry site marker broken reason
  for entry in "${home_markers[@]}"; do
    complete+="${entry#*|}"$'\n'
  done
  if ! home_body_error "$complete" >/dev/null; then
    echo "  SELF-TEST FAIL: the complete Home recording was refused" >&2
    failures=1
  fi
  if home_body_error "" >/dev/null 2>&1; then
    echo "  SELF-TEST FAIL: a blank 200 body was accepted as Home" >&2
    failures=1
  else
    echo "  ok: blank 200 -> refused"
  fi
  for entry in "${home_markers[@]}"; do
    site="${entry%%|*}"
    marker="${entry#*|}"
    broken="${complete//"$marker"/}"
    if reason="$(home_body_error "$broken")"; then
      echo "  SELF-TEST FAIL: Home without ${site} was accepted" >&2
      failures=1
    elif [ "$reason" != "missing ${site} marker: ${marker}" ]; then
      echo "  SELF-TEST FAIL: Home without ${site} failed at '${reason}'" >&2
      failures=1
    else
      echo "  ok: Home without ${site} -> refused at ${site}"
    fi
  done

  [ "$failures" -eq 0 ] || fail "an HTTP smoke verdict accepted a recorded regression"
  echo "self-test passed: widened binds and incomplete Home bodies were refused at their named invariants"
}

if [ "${1:-}" = "--self-test" ]; then
  self_test
  exit 0
fi

out_of_contract_note="${here}/vault/Notes/out-of-contract.md"
fixture_contract="${here}/vault/System/schemas/vault-schema.toml"
out_of_contract_type="$(sed -n 's/^type: //p' "$out_of_contract_note")"
contract_types="$(sed -n 's/^type = //p' "$fixture_contract")"
[ -n "$out_of_contract_type" ] || fail "the out-of-contract fixture has no type"
[ -n "$contract_types" ] || fail "the fixture contract has no type enum"
case "$contract_types" in
*\"${out_of_contract_type}\"*)
  fail "the out-of-contract fixture type ${out_of_contract_type} is present in the fixture enum"
  ;;
esac

base="${YOMIHON_BASE:?smoke.sh needs a running server; start it with serve.sh}"
port="${YOMIHON_PORT:?smoke.sh needs a running server; start it with serve.sh}"
check_home_marker_table
body="$(mktemp "${TMPDIR:-/tmp}/yomihon-smoke.XXXXXX")"
trap 'rm -f "$body"' EXIT

# Each non-Home face: 200 after following redirects, plus a marker that proves the right
# page rendered rather than a blank 200.
assert_face() {
  local path="$1" marker="$2" code
  code="$(curl -fsSL -o "$body" -w '%{http_code}' "${base}${path}")" || fail "GET ${path} did not return success"
  [ "$code" = "200" ] || fail "GET ${path} status ${code}, want 200"
  grep -qF "$marker" "$body" || fail "GET ${path} rendered but is missing its marker: ${marker}"
  echo "ok: ${path}"
}

# Home is fetched without redirect following: a retired 302 must not borrow the
# README page's 200 and markers. Its body then proves every dashboard block and
# the fixture vault README are both present.
code="$(curl -fsS -o "$body" -w '%{http_code}' "${base}/")" || fail "GET / did not return success"
[ "$code" = "200" ] || fail "GET / status ${code}, want direct 200"
if ! reason="$(home_body_error "$(<"$body")")"; then
  fail "GET / rendered but is incomplete: ${reason}"
fi
echo "ok: /"
assert_face "/notes/Notes/alpha.md" "tortoise"
assert_face "/notes/Notes/out-of-contract.md" "out-of-contract reading sentinel"
assert_face "/syllabus/Maps/study.md" "<title>Study Path"
assert_face "/search?q=tortoise" 'href="/notes/Notes/alpha.md"'
assert_face "/search/results?q=tortoise" 'data-live-search-results'

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
