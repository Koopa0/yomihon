#!/usr/bin/env bash
# PostToolUse hook: Verify last commit message format after git commit.
# Warns on stderr if format is wrong. Post-commit cannot block (commit already happened),
# but the warning tells Claude to create a follow-up fixup commit.

source "$(dirname "$0")/parse-hook-input.sh"

input=$(cat)
command=$(parse_command "$input")

if [[ -z "$command" || "$command" != *"git commit"* ]]; then
    exit 0
fi

# Check the actual committed message
msg=$(git log -1 --format=%s 2>/dev/null || true)

if [[ -z "$msg" ]]; then
    exit 0
fi

if ! echo "$msg" | grep -qE '^(feat|fix|refactor|test|docs|chore|perf|checkpoint): [a-z]'; then
    echo "WARNING: Last commit message '$msg' doesn't match format '<type>: <lowercase description>'. Create a new fixup commit with the correct format. See .claude/rules/git-workflow.md." >&2
fi

exit 0
