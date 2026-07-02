#!/bin/bash
# SubagentStop hook — ensure reviewer agents include Next Step recommendation
# Exit 2 = block agent from stopping (force it to add missing section)
# Exit 0 = allow stop

input=$(cat)

# Extract agent output from JSON
output=$(echo "$input" | jq -r '.agentOutput // .output // ""' 2>/dev/null)
[ -z "$output" ] && exit 0

# Check for Next Step section
if ! echo "$output" | grep -qi "next step"; then
  echo "AGENT QUALITY GATE: Output missing 'Next Step' recommendation. Add it before finishing." >&2
  exit 2
fi

exit 0
