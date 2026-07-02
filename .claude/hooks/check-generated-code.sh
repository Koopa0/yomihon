#!/usr/bin/env bash
# PreToolUse hook: Block edits to generated code (sqlc, go generate).
# Exit 2 with reason on stderr if matched, exit 0 otherwise.

source "$(dirname "$0")/parse-hook-input.sh"

input=$(cat)
file_path=$(parse_file_path "$input")

if [[ -z "$file_path" ]]; then
    exit 0
fi

project_root=$(cd "$(dirname "$0")/../.." && pwd)
relative_path="${file_path#"$project_root"/}"

# Block edits to internal/db/*.go (sqlc generated)
if [[ "$relative_path" == internal/db/* && "$relative_path" == *.go ]]; then
    echo "BLOCKED: Files in internal/db/ are sqlc-generated. Modify .sql query files and run 'sqlc generate' instead." >&2
    exit 2
fi

# Block edits to files with generated header
if [[ -f "$file_path" ]] && head -3 "$file_path" | grep -q "Code generated.*DO NOT EDIT"; then
    echo "BLOCKED: This file is generated code (DO NOT EDIT header). Modify the source and re-generate." >&2
    exit 2
fi

exit 0
