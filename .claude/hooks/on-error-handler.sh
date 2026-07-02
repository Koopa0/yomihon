#!/usr/bin/env bash
# PostToolUseFailure hook (Bash): classify the failed command's output and
# suggest the right agent. The event itself means the tool call failed, so
# no exit-code guessing is needed — but keep a guard for defensive reuse.
# Non-blocking — always exits 0.

INPUT=$(cat)

if command -v jq &>/dev/null; then
    EXIT_CODE=$(echo "$INPUT" | jq -r '.tool_response.exit_code // .tool_result.exit_code // .exit_code // 1' 2>/dev/null)
    STDERR=$(echo "$INPUT" | jq -r '.tool_response.stderr // .tool_response.error // .tool_result.stderr // .stderr // ""' 2>/dev/null)
    STDOUT=$(echo "$INPUT" | jq -r '.tool_response.stdout // .tool_result.stdout // .stdout // ""' 2>/dev/null)
else
    exit 0
fi

# Defensive: if a zero exit code somehow reaches a failure event, do nothing
[ "$EXIT_CODE" = "0" ] && exit 0

COMBINED="$STDOUT
$STDERR"

# Classify error and suggest agent
classify_error() {
    local output="$1"

    # Go build errors
    if echo "$output" | grep -qE 'cannot use|undefined:|declared and not used|too many arguments|not enough arguments'; then
        echo "[Error] Go build error detected" >&2
        echo "  Agent: @build-resolver" >&2
        return
    fi

    # Go type errors
    if echo "$output" | grep -qE 'cannot convert|mismatched types|invalid type assertion'; then
        echo "[Error] Go type error detected" >&2
        echo "  Suggestion: Check type compatibility" >&2
        echo "  Agent: @build-resolver" >&2
        return
    fi

    # golangci-lint errors
    if echo "$output" | grep -qiE 'golangci-lint|staticcheck|gosec|errcheck|gocritic'; then
        echo "[Error] Lint error detected" >&2
        echo "  Agent: @build-resolver" >&2
        return
    fi

    # Test failures
    if echo "$output" | grep -qiE 'FAIL|--- FAIL|panic:.*test'; then
        echo "[Error] Test failure detected" >&2
        echo "  Agent: @test-writer" >&2
        return
    fi

    # Import/module errors
    if echo "$output" | grep -qiE 'cannot find package|no required module provides|missing go.sum entry'; then
        echo "[Error] Module/import error detected" >&2
        echo "  Suggestion: Run 'go mod tidy'" >&2
        echo "  Agent: @build-resolver" >&2
        return
    fi

    # SQL/migration errors
    if echo "$output" | grep -qiE 'sqlc\|migration\|pgx\|pq:'; then
        echo "[Error] Database/SQL error detected" >&2
        echo "  Agent: @db-reviewer" >&2
        return
    fi
}

# Extract first few error lines for context
show_details() {
    local output="$1"
    local lines
    lines=$(echo "$output" | grep -iE 'error|fail|cannot|not found|undefined' | head -3)
    if [ -n "$lines" ]; then
        echo "  Details:" >&2
        echo "$lines" | while IFS= read -r line; do
            echo "    ${line:0:120}" >&2
        done
    fi
}

classify_error "$COMBINED"
show_details "$COMBINED"

exit 0
