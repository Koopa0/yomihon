#!/bin/bash
# PreToolUse hook: remind to run security-reviewer when touching security-critical files
# Advisory (exit 0) — reminds, does not block

source "$(dirname "$0")/parse-hook-input.sh"

input=$(cat)
FILE_PATH=$(parse_file_path "$input")

[[ -z "$FILE_PATH" ]] && exit 0
[[ "$FILE_PATH" != *.go ]] && exit 0

SECURITY_PATTERNS="auth|oauth|middleware|webhook|redirect|token|secret|crypto|hmac|password|session|csrf|cors"

if echo "$FILE_PATH" | grep -qiE "$SECURITY_PATTERNS"; then
  echo "SECURITY: Editing security-sensitive file '$(basename "$FILE_PATH")'. You MUST invoke security-reviewer agent before committing changes to this file." >&2
fi

exit 0
