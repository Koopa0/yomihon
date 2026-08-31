#!/usr/bin/env bash
# Runs a command against a live yomihon server bound to the fixture vault next
# to this script, then stops the server. Everything that needs a running server
# comes through here, so how the server is started — and above all how it is
# judged ready — is written down once. A second copy of the readiness rule is a
# copy that rots the day the home route answers differently.
#
#   serve.sh <yomihon-binary> <port> -- <command> [args...]
#
# The command runs with YOMIHON_BASE and YOMIHON_PORT exported, and its exit
# status becomes this script's. The server's log is printed only when something
# fails, so a green run stays quiet.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ready_status is the single readiness rule: Home answers directly only after
# the synchronous vault scan has completed and the routes are live. Callers do
# not restate the code; they ask this verdict.
ready_status() {
  [ "$1" = "200" ]
}

# The reading surface deliberately stays available when the vault contract is
# broken, so Home's 200 alone cannot prove that a browser fixture has a usable
# schema. Browser probes need the contract-backed navigation and write faces;
# require the real server startup log to say that contract loading succeeded.
contract_log_error() {
  local content="$1" loaded unavailable
  loaded="$(grep -cF 'msg="vault contract loaded"' <<<"$content" || true)"
  unavailable="$(grep -cF 'msg="vault contract unavailable; write face is closed (fail-closed)"' <<<"$content" || true)"
  if [ "$unavailable" -ne 0 ]; then
    echo "the server reported the fixture contract unavailable"
    return 1
  fi
  if [ "$loaded" -ne 1 ]; then
    echo "the server reported ${loaded} successful fixture contract loads, want exactly 1"
    return 1
  fi
}

# The readiness request must be answered by the child this script started, not
# by an unrelated process that already owned the requested port. The serving
# line is emitted only after yomihon has successfully bound its listener, so an
# exact single match is the ownership proof that an HTTP 200 alone cannot give.
serving_log_error() {
  local content="$1" address="$2" serving exact
  serving="$(grep -cF 'msg="yomihon serving"' <<<"$content" || true)"
  exact="$(grep -cF "msg=\"yomihon serving\" addr=${address} " <<<"$content" || true)"
  if [ "$serving" -ne 1 ] || [ "$exact" -ne 1 ]; then
    echo "the child reported ${serving} serving lines and ${exact} exact binds for ${address}, want exactly 1 of each"
    return 1
  fi
}

copy_fixture() {
  cp -R "$1" "$2"
}

# The cases exercise the verdict; the required set is separate so removing a
# redirect or missing-route control cannot weaken the self-test silently.
readiness_cases=(
  '200|accepted'
  '302|refused'
  '404|refused'
  '000|refused'
)
required_readiness_codes=(200 302 404 000)

check_readiness_cases() {
  local actual required
  actual="$(printf '%s\n' "${readiness_cases[@]%%|*}" | sort)"
  required="$(printf '%s\n' "${required_readiness_codes[@]}" | sort)"
  [ "$actual" = "$required" ] || {
    echo "serve.sh: readiness controls differ from the required 200, 302, 404, and no-response set" >&2
    exit 1
  }
}

# A live yomihon can only show the accepted case, so recorded status codes prove
# the verdict refuses the two regressions most likely to masquerade as startup:
# the retired redirect and a missing route.
self_test() {
  local failures=0 entry code want got reason
  check_readiness_cases
  for entry in "${readiness_cases[@]}"; do
    code="${entry%%|*}"
    want="${entry#*|}"
    if ready_status "$code"; then got=accepted; else got=refused; fi
    if [ "$got" != "$want" ]; then
      echo "  SELF-TEST FAIL: HTTP ${code} was ${got}, want ${want}" >&2
      failures=1
    else
      echo "  ok: HTTP ${code} -> ${got}"
    fi
  done

  if ! contract_log_error 'time=2026-07-17T00:00:00Z level=INFO msg="vault contract loaded" version=1 lifecycle_stages=2'; then
    echo "  SELF-TEST FAIL: one successful contract load was refused" >&2
    failures=1
  else
    echo "  ok: one successful contract load -> accepted"
  fi
  for entry in \
    'unavailable|the server reported the fixture contract unavailable|time=2026-07-17T00:00:00Z level=WARN msg="vault contract unavailable; write face is closed (fail-closed)" error=invalid' \
    'missing|reported 0 successful fixture contract loads|time=2026-07-17T00:00:00Z level=INFO msg="scanner ready"' \
    'duplicated|reported 2 successful fixture contract loads|time=2026-07-17T00:00:00Z level=INFO msg="vault contract loaded" version=1
time=2026-07-17T00:00:01Z level=INFO msg="vault contract loaded" version=1'; do
    code="${entry%%|*}"
    entry="${entry#*|}"
    want="${entry%%|*}"
    entry="${entry#*|}"
    if reason="$(contract_log_error "$entry")"; then
      echo "  SELF-TEST FAIL: ${code} contract log was accepted" >&2
      failures=1
    elif [[ "$reason" != *"$want"* ]]; then
      echo "  SELF-TEST FAIL: ${code} contract log failed as ${reason}, want ${want}" >&2
      failures=1
    else
      echo "  ok: ${code} contract log -> ${reason}"
    fi
  done

  if ! serving_log_error 'time=2026-07-17T00:00:00Z level=INFO msg="yomihon serving" addr=127.0.0.1:19733 vault=/tmp/fixture' '127.0.0.1:19733'; then
    echo "  SELF-TEST FAIL: one exact listener bind was refused" >&2
    failures=1
  else
    echo "  ok: one exact listener bind -> accepted"
  fi
  for entry in \
    'missing|reported 0 serving lines|time=2026-07-17T00:00:00Z level=INFO msg="vault contract loaded" version=1' \
    'wrong-address|0 exact binds|time=2026-07-17T00:00:00Z level=INFO msg="yomihon serving" addr=127.0.0.1:19734 vault=/tmp/fixture' \
    'duplicated|reported 2 serving lines|time=2026-07-17T00:00:00Z level=INFO msg="yomihon serving" addr=127.0.0.1:19733 vault=/tmp/fixture
time=2026-07-17T00:00:01Z level=INFO msg="yomihon serving" addr=127.0.0.1:19733 vault=/tmp/fixture'; do
    code="${entry%%|*}"
    entry="${entry#*|}"
    want="${entry%%|*}"
    entry="${entry#*|}"
    if reason="$(serving_log_error "$entry" '127.0.0.1:19733')"; then
      echo "  SELF-TEST FAIL: ${code} listener log was accepted" >&2
      failures=1
    elif [[ "$reason" != *"$want"* ]]; then
      echo "  SELF-TEST FAIL: ${code} listener log failed as ${reason}, want ${want}" >&2
      failures=1
    else
      echo "  ok: ${code} listener log -> ${reason}"
    fi
  done

  if (
    set -e
    fixture_test="$(mktemp -d "${TMPDIR:-/tmp}/yomihon-fixture-copy.XXXXXX")"
    trap 'rm -rf "$fixture_test"' EXIT
    mkdir "$fixture_test/source"
    printf '%s\n' original >"$fixture_test/source/marker"
    copy_fixture "$fixture_test/source" "$fixture_test/copy"
    printf '%s\n' changed >"$fixture_test/copy/marker"
    [ "$(<"$fixture_test/source/marker")" = original ]
  ); then
    echo "  ok: disposable fixture writes do not reach the checked-in source"
  else
    echo "  SELF-TEST FAIL: the browser fixture copy aliases its source" >&2
    failures=1
  fi
  [ "$failures" -eq 0 ] || { echo "serve.sh: startup self-test accepted an incomplete server state" >&2; exit 1; }
  echo "self-test passed: readiness, contract, and listener-ownership checks refuse incomplete server startup"
}

if [ "${1:-}" = "--self-test" ]; then
  self_test
  exit 0
fi

if [ "$#" -lt 3 ] || [ "$3" != "--" ]; then
  echo "usage: serve.sh <yomihon-binary> <port> -- <command> [args...]" >&2
  exit 2
fi
bin="$1"
port="$2"
shift 3
if [ "$#" -eq 0 ]; then
  echo "serve.sh: nothing to run after --" >&2
  exit 2
fi

base="http://127.0.0.1:${port}"
# Every browser run gets a private copy. A behavior probe may exercise POST
# paths, and a failed invariant must not leave the checked-in fixture changed.
# The source remains the canonical reviewable fixture; the server sees only the
# disposable copy beneath this named temporary directory.
work="$(mktemp -d "${TMPDIR:-/tmp}/yomihon-serve.XXXXXX")"
vault="$work/vault"
log="$work/server.log"
server_pid=""
# shellcheck disable=SC2329 # Invoked indirectly by the EXIT trap below.
cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2>/dev/null || true
    for _ in $(seq 1 70); do
      kill -0 "$server_pid" 2>/dev/null || break
      sleep 0.1
    done
    if kill -0 "$server_pid" 2>/dev/null; then
      kill -KILL "$server_pid" 2>/dev/null || true
    fi
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT

# The probes read the fixture beside this script. The screenshot target reads
# the example vault instead, because a picture of a fixture would be a picture
# of something no reader can open. Everything else about the run — above all how
# readiness is judged — stays the same for both.
copy_fixture "${YOMIHON_FIXTURE:-$here/vault}" "$vault"

YOMIHON_PORT="$port" "$bin" serve "$vault" >"$log" 2>&1 &
server_pid=$!

dump_log() {
  if [ -s "$log" ]; then
    echo "--- server log ---" >&2
    cat "$log" >&2
  fi
}

# Readiness: Home answers directly with 200 only once the vault has been scanned
# and the routes are live.
# Polling a served answer rather than the open socket is what proves the server
# can serve, not merely that it has bound a port.
#
# A server that has already exited will never answer, so the loop asks whether it
# is still alive before it asks whether it is ready. Otherwise a bind that failed
# in the first millisecond costs fifteen seconds of polling and reports a timeout,
# which reads as a slow server rather than a dead one.
ready=""
code="000"
for _ in $(seq 1 60); do
  if ! kill -0 "$server_pid" 2>/dev/null; then
    echo "serve.sh: the server exited before it answered on ${base}" >&2
    dump_log
    exit 1
  fi
  # A bound port is not a promise of an answer: something else listening there
  # and staying silent would hold an untimed request open until the whole job
  # expired, with nothing said about why. Each poll gives up quickly instead.
  code="$(curl -s -o /dev/null --connect-timeout 1 --max-time 2 -w '%{http_code}' "${base}/" || true)"
  if ready_status "$code"; then
    ready=1
    break
  fi
  sleep 0.25
done
if [ -z "$ready" ]; then
  echo "serve.sh: the server never answered Home's direct 200 readiness signal on ${base} (last HTTP status: ${code})" >&2
  dump_log
  exit 1
fi

# Ownership is asked first because it explains the other answer. A foreign
# process already holding the port answers Home's 200 while the child this
# script started never binds and logs nothing, and an empty log reads to the
# contract check as zero successful loads — a port conflict reported as a
# broken fixture schema. Neither ordering can turn a foreign 200 into a pass;
# this one names the fault the operator has to fix.
if ! reason="$(serving_log_error "$(<"$log")" "127.0.0.1:${port}")"; then
  echo "serve.sh: listener ownership preflight failed: ${reason}" >&2
  dump_log
  exit 1
fi
if ! reason="$(contract_log_error "$(<"$log")")"; then
  echo "serve.sh: fixture contract preflight failed: ${reason}" >&2
  dump_log
  exit 1
fi

export YOMIHON_BASE="$base" YOMIHON_PORT="$port"
if "$@"; then
  status=0
else
  status=$?
fi
[ "$status" -eq 0 ] || dump_log
exit "$status"
