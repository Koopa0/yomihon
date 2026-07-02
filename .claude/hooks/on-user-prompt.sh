#!/usr/bin/env bash
# UserPromptSubmit hook: Validate user input before processing.
# Blocks dangerous patterns. Non-blocking for normal prompts.

INPUT=$(cat)

if command -v jq &>/dev/null; then
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // ""' 2>/dev/null)
else
    exit 0
fi

[ -z "$PROMPT" ] && exit 0

# Block patterns that could cause data loss
PROMPT_LOWER=$(echo "$PROMPT" | tr '[:upper:]' '[:lower:]')

# Block requests to delete entire directories or databases
if echo "$PROMPT_LOWER" | grep -qE 'delete (all|everything|the (entire|whole)|every).*(\bfile|\bdir|\btable|\bdata|\brecord)'; then
    echo '{"decision":"block","reason":"Blocked: mass deletion request detected. Please be more specific about what to delete."}'
    exit 0
fi

# Warn about force push requests
if echo "$PROMPT_LOWER" | grep -qE 'force push|push.*(--force|-f\b)'; then
    echo '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"WARNING: User is requesting a force push. Confirm the branch and verify no shared work will be lost before proceeding."}}'
    exit 0
fi

# Autonomous skill surfacing: the user should not have to remember which review
# skill to invoke (manual /commands decay into disuse). On a CLEAR signal,
# surface the relevant skill via additionalContext (advisory, never blocks).
# Patterns are high-precision to avoid alert fatigue.
if echo "$PROMPT_LOWER" | grep -qE 'over.?engineer|too complex|am i (doing|over)|did i go too far|is this (right|overkill|too much)|sanity check|roast|challenge (this|my)|devil.?s? advocate|find (the )?problems'; then
    echo '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"SKILL SUGGESTION: this reads as a post-decision critique — consider the /devil-advocate skill (adversarial retroactive review: should this exist, is it too complex, is it honest)."}}'
    exit 0
fi
if echo "$PROMPT_LOWER" | grep -qE 'design review|review the design|architecture review|does (this|the).*(package|structure|architecture|naming).*(make sense|right)|is the naming'; then
    echo '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"SKILL SUGGESTION: this reads as a design-quality question — consider the /design-review skill (package purpose, naming concepts, new-reader confusion)."}}'
    exit 0
fi
# New-feature signal → reinforce the "comprehend FIRST" mandate (agents.md).
# Requires a feature/package/service noun so trivial edits ("add a status
# field", "add a column") do NOT fire; message is tier-conditional so it
# self-limits to genuine Tier 3 (new package) work.
if echo "$PROMPT_LOWER" | grep -qE 'new (feature|package|service)|(add|implement|build|create) (a |an |the )?[a-z0-9 ]*(feature|package|service|new endpoint)'; then
    echo '{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"WORKFLOW: if this introduces a NEW package / Tier 3 work, run the comprehend agent FIRST (agents.md auto-delegation) before implementing — it challenges the request and reads the codebase. Skip for Tier 1/2 edits to existing packages."}}'
    exit 0
fi

exit 0
