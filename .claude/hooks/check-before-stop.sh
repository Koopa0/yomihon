#!/usr/bin/env bash
# Stop hook — make the Completion Criteria machine-enforced.
#
# BLOCKS (exit 2) when uncommitted .go changes fail any stage the git-workflow
# rule already requires before a commit: build -> vet -> test -> lint. The
# stderr reason becomes Claude's NEXT prompt, so it names the exact failing
# package/test/linter rather than "tests fail" — Claude can act surgically
# instead of re-running everything.
#
# Loop-safety: Claude Code sets stop_hook_active=true on the stdin payload when
# a Stop hook already fired this stretch. We exit 0 immediately in that case so
# a wedged session can always recover (and Claude force-ends after 8 blocks).

input=$(cat 2>/dev/null || true)

# Loop guard: never block twice in a row.
stop_active=""
if command -v jq &>/dev/null; then
    stop_active=$(printf '%s' "$input" | jq -r '.stop_hook_active // empty' 2>/dev/null)
else
    printf '%s' "$input" | grep -q '"stop_hook_active"[[:space:]]*:[[:space:]]*true' && stop_active="true"
fi
[[ "$stop_active" == "true" ]] && exit 0

changed=$(git diff --name-only 2>/dev/null | wc -l | tr -d ' ')
go_changed=$(git diff --name-only 2>/dev/null | grep -c '\.go$' || true)

# The build/vet/test/lint gate runs only when uncommitted .go changes exist;
# the commit reminders and the /reflect nudge below always run (a clean tree
# can still have learnings to promote), so do NOT early-exit here.

if [[ "${go_changed:-0}" -gt 0 ]]; then
    # 1. build
    if ! build_err=$(go build ./... 2>&1); then
        first=$(printf '%s' "$build_err" | grep -vE '^#' | head -1)
        echo "BLOCKED: build fails — fix before stopping. First error: ${first:-see go build ./...}" >&2
        exit 2
    fi

    # 2. vet
    if ! vet_err=$(go vet ./... 2>&1); then
        first=$(printf '%s' "$vet_err" | grep -vE '^#' | head -1)
        echo "BLOCKED: go vet fails — fix before stopping. First issue: ${first:-see go vet ./...}" >&2
        exit 2
    fi

    # 3. test — name the failing tests via the JSON event stream.
    if ! test_json=$(go test -json ./... 2>/dev/null); then
        if command -v jq &>/dev/null; then
            fails=$(printf '%s' "$test_json" | jq -r 'select(.Action=="fail" and .Test!=null) | "\(.Test) (\(.Package))"' 2>/dev/null | sort -u | head -8 | paste -sd', ' -)
        else
            fails=$(printf '%s' "$test_json" | grep '"Action":"fail"' | grep -o '"Test":"[^"]*"' | sed 's/"Test":"//;s/"//' | sort -u | head -8 | paste -sd', ' -)
        fi
        echo "BLOCKED: tests fail — fix before stopping. Failing: ${fails:-run go test ./... to see}." >&2
        exit 2
    fi

    # 4. lint — only reached once build/vet/test are green.
    if command -v golangci-lint &>/dev/null; then
        if ! lint_out=$(golangci-lint run ./... 2>&1); then
            n=$(printf '%s' "$lint_out" | grep -cE ':[0-9]+:[0-9]+:' || true)
            first=$(printf '%s' "$lint_out" | grep -E ':[0-9]+:[0-9]+:' | head -1)
            echo "BLOCKED: golangci-lint reports ${n:-1} issue(s) — zero-tolerance before stopping. First: ${first:-run golangci-lint run ./...}" >&2
            exit 2
        fi
    fi
fi

# Pass path: non-blocking reminders.
if ! git diff --quiet 2>/dev/null; then
    echo "REMINDER: $changed file(s) have uncommitted changes. Commit before stopping." >&2
fi
if ! git diff --cached --quiet 2>/dev/null; then
    echo "REMINDER: Staged changes not yet committed." >&2
fi

# Autonomous /reflect nudge: if learnings were captured this session, the user
# shouldn't have to remember to promote them. Fires only when the append-only
# log has content (gitignored; populated by the reflect skill's capture step).
learnings_log="$(cd "$(dirname "$0")/../.." && pwd)/.claude/session-learnings.log"
if [ -s "$learnings_log" ]; then
    n=$(grep -c '^candidate-target:' "$learnings_log" 2>/dev/null || echo "some")
    echo "REMINDER: $n learning(s) captured this session in session-learnings.log — run /reflect to review them for promotion (human-gated)." >&2
fi

exit 0
