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

# Readiness: the home path answers with a redirect to the vault's reading page,
# and it does so only once the vault has been scanned and the routes are live.
# Polling a served answer rather than the open socket is what proves the server
# can serve, not merely that it has bound a port.
#
# A server that has already exited will never answer, so the loop asks whether it
# is still alive before it asks whether it is ready. Otherwise a bind that failed
# in the first millisecond costs fifteen seconds of polling and reports a timeout,
# which reads as a slow server rather than a dead one.
ready=""
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
  if [ "$code" = "302" ]; then
    ready=1
    break
  fi
  sleep 0.25
done
if [ -z "$ready" ]; then
  echo "serve.sh: the server never answered the reading redirect on ${base}" >&2
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
