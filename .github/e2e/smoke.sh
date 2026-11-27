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
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

# One named site per way into the library, one content marker that only the
# fixture vault's rendered README can supply, and the quiet row that leads from
# the desk to the library's health.
#
# The rail is not among them. Home carries the four ways in and no rail: a
# listing beside a copy of itself is a second organisation of the same notes,
# so what the rail used to promise here is promised on the note page below.
home_markers=(
  'courses|data-home-block="courses"'
  'indexes|data-home-block="indexes"'
  'reports|data-home-block="reports"'
  'folders|data-home-block="folders"'
  'vault-readme|data-home-readme'
  'desk-health|data-home-health'
)

# The rail, checked where it now lives: beside the thing being read. The map
# tree marker is what proves a tree rendered rather than an empty group.
note_markers=(
  'sidebar-course|data-sidebar-mode="course"'
  'sidebar-course-path|data-book-path="Maps/study.md"'
  'sidebar-course-current|aria-current="page"'
)

note_body_error() {
  local content="$1" entry site marker
  for entry in "${note_markers[@]}"; do
    site="${entry%%|*}"
    marker="${entry#*|}"
    if ! grep -qF "$marker" <<<"$content"; then
      echo "missing ${site} marker: ${marker}"
      return 1
    fi
  done
}

# Reads a vault contract on stdin and prints each note type it declares, one
# per line: the name of every [types.<name>] table, and every word of the type
# list under [enums], which the loader joins to the tables. A type word written
# under any other key or table is not a declaration, so a list of levels or of
# domains cannot make a type look declared.
declared_types() {
  awk '
    function words(text,    rest) {
      rest = text
      while (match(rest, /"[^"]*"/)) {
        print substr(rest, RSTART + 1, RLENGTH - 2)
        rest = substr(rest, RSTART + RLENGTH)
      }
      return rest ~ /\]/
    }
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*\[/ {
      header = $0
      sub(/^[[:space:]]*\[[[:space:]]*/, "", header)
      sub(/[[:space:]]*\].*$/, "", header)
      in_enums = (header == "enums")
      listing = 0
      if (header ~ /^types\./) {
        name = header
        sub(/^types\./, "", name)
        gsub(/"/, "", name)
        print name
      }
      next
    }
    in_enums && !listing && /^[[:space:]]*type[[:space:]]*=/ {
      line = $0
      sub(/^[^=]*=/, "", line)
      listing = !words(line)
      next
    }
    in_enums && listing { listing = !words($0) }
  '
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
# at all, which is not the promise either. The reason is echoed for the caller
# to print.
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

out_of_contract_note="${here}/vault/Notes/out-of-contract.md"
fixture_contract="${here}/vault/System/schemas/vault-schema.toml"
out_of_contract_type="$(sed -n 's/^type: //p' "$out_of_contract_note")"
contract_types="$(declared_types <"$fixture_contract")"
[ -n "$out_of_contract_type" ] || fail "the out-of-contract fixture has no type"
[ -n "$contract_types" ] || fail "the fixture contract declares no note type"
if grep -qxF -- "$out_of_contract_type" <<<"$contract_types"; then
  fail "the out-of-contract fixture type ${out_of_contract_type} is declared by the fixture contract"
fi

base="${YOMIHON_BASE:?smoke.sh needs a running server; start it with serve.sh}"
port="${YOMIHON_PORT:?smoke.sh needs a running server; start it with serve.sh}"
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

# The retired addresses of a course answer with a redirect to its own, carrying
# the rest of the address and the query: fetched without following, so a page
# still served at the old address cannot pass for the redirect.
assert_redirect() {
  local path="$1" want="$2" result
  result="$(curl -sS -o /dev/null -w '%{http_code} %{redirect_url}' "${base}${path}")" || fail "GET ${path} did not return"
  [ "$result" = "301 ${base}${want}" ] || fail "GET ${path} answered ${result}, want 301 ${base}${want}"
  echo "ok: ${path} moved to ${want}"
}

# Home is fetched without redirect following: a retired 302 must not borrow the
# README page's 200 and markers. Its body then proves every block on Home and
# the fixture vault README are both present.
code="$(curl -fsS -o "$body" -w '%{http_code}' "${base}/")" || fail "GET / did not return success"
[ "$code" = "200" ] || fail "GET / status ${code}, want direct 200"
if ! reason="$(home_body_error "$(<"$body")")"; then
  fail "GET / rendered but is incomplete: ${reason}"
fi
echo "ok: /"
assert_face "/notes/Notes/alpha.md" "tortoise"
if ! reason="$(note_body_error "$(<"$body")")"; then
  fail "GET /notes/Notes/alpha.md rendered but its rail is incomplete: ${reason}"
fi
echo "ok: the rail on /notes/Notes/alpha.md"
assert_face "/notes/Notes/out-of-contract.md" "out-of-contract reading sentinel"
assert_face "/courses/Maps/study.md" "<title>Study Path"
assert_face "/courses" 'data-index="courses"'
assert_redirect "/syllabus/Maps/study.md?from=Notes%2Falpha.md" "/courses/Maps/study.md?from=Notes%2Falpha.md"
assert_redirect "/paths" "/courses"
assert_face "/indexes" 'data-index="indexes"'
assert_redirect "/maps" "/indexes"
assert_redirect "/open-thoughts" "/marks"
assert_face "/search?q=tortoise" 'class="y-found__item" href="/notes/Notes/alpha.md'
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
