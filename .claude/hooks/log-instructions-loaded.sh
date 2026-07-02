#!/usr/bin/env bash
# InstructionsLoaded hook (async, observe-only): record which CLAUDE.md /
# rule files actually loaded, when, and why — a deterministic ground-truth
# log of governance loading. Turns "did the rules load?" from a guess into
# evidence (addresses the recurring "is it really following our rules?"
# worry), and diagnoses cold-write gaps: if a *store*.go is created and
# database.md never logs a path_glob_match, the rule didn't inject.
#
# Appends JSONL to the gitignored .claude/rule-load.log. Never blocks
# (the event has no decision control); always exits 0.

source "$(dirname "$0")/parse-hook-input.sh" 2>/dev/null || true

input=$(cat)
log_file="$(cd "$(dirname "$0")/../.." && pwd)/.claude/rule-load.log"

if command -v jq &>/dev/null; then
    line=$(printf '%s' "$input" | jq -c '{
        file: (.file_path // ""),
        memory: (.memory_type // ""),
        reason: (.load_reason // ""),
        globs: (.globs // null),
        trigger: (.trigger_file_path // null)
    }' 2>/dev/null)
    [ -n "$line" ] && printf '%s\n' "$line" >> "$log_file" 2>/dev/null
fi

exit 0
