#!/usr/bin/env bash
# Runs a command against a live yomihon server bound to the fixture vault next
# to this script, then stops the server. Everything that needs a running server
# comes through here, so how the server is started — and above all how it is
# judged ready — is written down once. A second copy of the readiness rule is a
# copy that rots the day the home route answers differently.
#
#   serve.sh <yomihon-binary> <port> -- <command> [args...]
#
# Port 0 lets the system choose a free loopback port, so runs in separate
# worktrees never contend for one; the port is read back from the child's own
# serving line.
#
# The command runs with YOMIHON_BASE and YOMIHON_PORT exported, and its exit
# status becomes this script's. The server's log is printed only when something
# fails, so a green run stays quiet.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ready_status is the single readiness rule: Home answers 200 only after the
# first read of the vault has completed and the routes are live; until then the
# server is already listening and answers 503 with the page that says so.
# Callers do not restate the code; they ask this verdict.
ready_status() {
  [ "$1" = "200" ]
}

# The reading surface deliberately stays available when the vault contract is
# broken, so Home's 200 alone cannot prove that a browser fixture has a usable
# schema. Browser probes need the contract-backed navigation;
# require the real server startup log to say that contract loading succeeded.
contract_log_error() {
  local content="$1" loaded unavailable
  loaded="$(grep -cF 'msg="rules file loaded"' <<<"$content" || true)"
  unavailable="$(grep -cF 'msg="rules file unavailable"' <<<"$content" || true)"
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

# The port a server asked for port 0 was given, read from its own serving line.
# Empty until that line appears, and for a line naming any address but loopback.
bound_port() {
  sed -n 's/.*msg="yomihon serving" addr=127\.0\.0\.1:\([0-9][0-9]*\) .*/\1/p' <<<"$1" | head -n 1
}

copy_fixture() {
  cp -R "$1" "$2"
}

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

# The server keeps the reader's own marks under the configuration directory
# this platform gives it, which the standard library resolves from HOME (and
# XDG_CONFIG_HOME where the platform has one). A probe run that inherited the
# operator's own would write a mark into the directory they actually read
# with, so the child is given a home of its own beneath the disposable work
# directory and the run leaves nothing behind.
home="$work/home"
mkdir -p "$home"

HOME="$home" XDG_CONFIG_HOME="$home/.config" YOMIHON_PORT="$port" "$bin" serve "$vault" >"$log" 2>&1 &
server_pid=$!

dump_log() {
  if [ -s "$log" ]; then
    echo "--- server log ---" >&2
    cat "$log" >&2
  fi
}

# Asked for port 0, the child binds one the system chose and says which in its
# serving line. Reading it from the child's own log keeps the address one this
# run owns: nothing was picked beforehand for another process to take.
if [ "$port" = 0 ]; then
  bound=""
  for _ in $(seq 1 60); do
    if ! kill -0 "$server_pid" 2>/dev/null; then
      echo "serve.sh: the server exited before it reported the port it bound" >&2
      dump_log
      exit 1
    fi
    bound="$(bound_port "$(<"$log")")"
    [ -z "$bound" ] || break
    sleep 0.25
  done
  if [ -z "$bound" ]; then
    echo "serve.sh: the server never reported the port it bound" >&2
    dump_log
    exit 1
  fi
  port="$bound"
fi
base="http://127.0.0.1:${port}"

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

# Browser probes may change notes only inside this owned disposable copy.
export YOMIHON_FIXTURE_ROOT="$vault"
export YOMIHON_BASE="$base" YOMIHON_PORT="$port"
# Only developer probes receive the disposable fixture path, never the server.
export YOMIHON_FIXTURE_ROOT="$vault"
# The server's own disposable home, so a probe can place the one record the
# server reads and no route writes: a figure approval, which in use only a
# person at a terminal gives.
export YOMIHON_FIXTURE_HOME="$home"
if "$@"; then
  status=0
else
  status=$?
fi
[ "$status" -eq 0 ] || dump_log
exit "$status"
