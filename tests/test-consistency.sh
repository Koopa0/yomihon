#!/usr/bin/env bash
# Rule consistency checker (kurodo edition): validates file existence and
# structural integrity of the harness. Adapted from go-spec's version —
# kurodo has no Dockerfile, no .agents/ mirror (AGENTS.md is a pointer),
# and its entry point is cmd/kurodo.
# Run from project root: bash tests/test-consistency.sh
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

PASS=0
FAIL=0
TOTAL=0

red()   { printf "\033[31m%s\033[0m\n" "$1"; }
green() { printf "\033[32m%s\033[0m\n" "$1"; }
bold()  { printf "\033[1m%s\033[0m\n" "$1"; }

check() {
    local description="$1"
    local condition="$2"  # "pass" or "fail"
    TOTAL=$((TOTAL + 1))
    if [[ "$condition" == "pass" ]]; then
        green "  PASS: $description"
        PASS=$((PASS + 1))
    else
        red "  FAIL: $description"
        FAIL=$((FAIL + 1))
    fi
}

# ============================================================
bold "=== File Structure ==="
# ============================================================

for dir in cmd/kurodo internal migrations .claude/rules .claude/agents .claude/hooks .claude/skills; do
    if [[ -d "$dir" ]]; then
        check "directory exists: $dir" "pass"
    else
        check "directory exists: $dir" "fail"
    fi
done

for file in go.mod sqlc.yaml Makefile .gitignore .golangci.yml CLAUDE.md cmd/kurodo/main.go .claude/QUICKSTART.md docs/vault-model.md docs/design.md docs/decisions.md; do
    if [[ -f "$file" ]]; then
        check "file exists: $file" "pass"
    else
        check "file exists: $file" "fail"
    fi
done

# ============================================================
bold "=== Agent ↔ CLAUDE.md Consistency ==="
# ============================================================

for agent in comprehend planner scaffold go-reviewer test-writer build-resolver db-reviewer review-code; do
    if [[ -f ".claude/agents/$agent.md" ]]; then
        check "agent file exists: $agent.md" "pass"
    else
        check "agent file exists: $agent.md" "fail"
    fi
done

for agent_file in .claude/agents/*.md; do
    name=$(basename "$agent_file" .md)
    if grep -q "^name:" "$agent_file"; then
        check "agent $name has 'name' frontmatter" "pass"
    else
        check "agent $name has 'name' frontmatter" "fail"
    fi
    if grep -q "^tools:" "$agent_file"; then
        check "agent $name has 'tools' frontmatter" "pass"
    else
        check "agent $name has 'tools' frontmatter" "fail"
    fi
done

# ============================================================
bold "=== Skills ==="
# ============================================================

for skill in verify checkpoint debug pgx-patterns sqlc-guide postgres-patterns migrations http-server test-strategy lifecycle-phases; do
    if [[ -f ".claude/skills/$skill/SKILL.md" ]]; then
        check "skill exists: $skill" "pass"
    else
        check "skill exists: $skill" "fail"
    fi
done

# ============================================================
bold "=== sqlc Configuration ==="
# ============================================================

if grep -q "sql_package: \"pgx/v5\"" sqlc.yaml || grep -q "sql_package: pgx/v5" sqlc.yaml; then
    check "sqlc sql_package is pgx/v5" "pass"
else
    check "sqlc sql_package is pgx/v5" "fail"
fi

# ============================================================
bold "=== Makefile Targets ==="
# ============================================================

for target in build run test lint fmt vet gen css sqlc verify verify-spec clean; do
    if grep -q "^${target}:" Makefile; then
        check "Makefile has target: $target" "pass"
    else
        check "Makefile has target: $target" "fail"
    fi
done

# ============================================================
bold "=== Go Build Verification ==="
# ============================================================

if go build ./... 2>/dev/null; then
    check "go build ./... succeeds" "pass"
else
    check "go build ./... succeeds" "fail"
fi

# ============================================================
bold "=== Summary ==="
# ============================================================
echo ""
echo "Total: $TOTAL, Pass: $PASS, Fail: $FAIL"
if [[ $FAIL -gt 0 ]]; then
    exit 1
fi
