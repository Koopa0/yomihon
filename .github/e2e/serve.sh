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
  local failures=0 entry code want got
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
  [ "$failures" -eq 0 ] || { echo "serve.sh: readiness accepts a response other than Home's direct 200" >&2; exit 1; }
  echo "self-test passed: readiness accepts Home's direct 200 and refuses redirect, missing, and no-response signals"
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

vault="$here/vault"
base="http://127.0.0.1:${port}"
# Named, and with a template. Every documented form of mktemp on this machine's
# BSD takes one; a bare call is tolerated rather than promised, and these scripts
# are meant to run here as well as on the runner. The name is what identifies the
# file if one is ever left behind.
log="$(mktemp "${TMPDIR:-/tmp}/yomihon-serve.XXXXXX")"

YOMIHON_ROOT="$vault" YOMIHON_PORT="$port" "$bin" serve >"$log" 2>&1 &
server_pid=$!
cleanup() {
  kill "$server_pid" 2>/dev/null || true
  wait "$server_pid" 2>/dev/null || true
  rm -f "$log"
}
trap cleanup EXIT

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

export YOMIHON_BASE="$base" YOMIHON_PORT="$port"
if "$@"; then
  status=0
else
  status=$?
fi
[ "$status" -eq 0 ] || dump_log
exit "$status"
