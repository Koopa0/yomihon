#!/usr/bin/env bash
# SubagentStop hook: validate planner output format.
# Matcher in settings.json already filters to "planner" only.
# Exit 0 = allow (with warnings on stderr). Not blocking.

input=$(cat)

# Extract agent output from JSON (same pattern as check-agent-output.sh)
OUTPUT=$(echo "$input" | jq -r '.agentOutput // .output // ""' 2>/dev/null)
[ -z "$OUTPUT" ] && exit 0

# Check for Implementation Tasks (existing requirement)
if ! echo "$OUTPUT" | grep -qi "Implementation Tasks"; then
  echo "WARNING: Planner output missing 'Implementation Tasks' section." >&2
fi

# Check for comparison table OR single path declaration
HAS_COMPARISON=false
HAS_SINGLE_PATH=false

if echo "$OUTPUT" | grep -qiE "(方案 [AB]|Option [AB]|Plan [AB])"; then
  HAS_COMPARISON=true
fi

if echo "$OUTPUT" | grep -qiE "(Single Path|為什麼沒有替代方案|Single Path Rationale|no alternative)"; then
  HAS_SINGLE_PATH=true
fi

if [[ "$HAS_COMPARISON" == "false" && "$HAS_SINGLE_PATH" == "false" ]]; then
  echo "WARNING: Planner output has neither a comparison table nor a Single Path declaration. Consider adding one." >&2
fi

exit 0
