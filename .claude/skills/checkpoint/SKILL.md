---
name: checkpoint
description: >-
  Creates a git checkpoint commit to save work-in-progress, giving an easy
  rollback point — build-verified, specific files staged (never git add .),
  committed with a checkpoint message.
when_to_use: >-
  Use before risky changes — large refactors, changing shared types or
  interfaces, modifying database queries or migrations, or any change touching
  many files — or whenever a save point to roll back to is wanted. Invoked as
  /checkpoint [description].
disable-model-invocation: true
argument-hint: "[description]"
metadata:
  author: koopa
  version: "1.1"
  lang: go
---

# /checkpoint — Git Checkpoint

Create a checkpoint commit to save current work-in-progress before making risky changes. This allows easy rollback if something goes wrong.

## When to Use

- Before a large refactor
- Before changing shared types or interfaces
- Before modifying database queries or migrations
- Before any change that touches many files
- When you want a save point to rollback to

## Process

### Step 1: Verify Clean State
```bash
go build ./...
```
Only checkpoint code that compiles. If build fails, fix first.

### Step 2: Check What Changed
```bash
git status
git diff --stat
```
Review the changes to write an accurate checkpoint message.

### Step 3: Stage and Commit
Stage specific files (NEVER `git add .`):
```bash
git add <specific files>
git commit -m "checkpoint: <description of current state>"
```

The checkpoint message format:
```
checkpoint: <what was done so far>

Work-in-progress. Safe rollback point before <what comes next>.
```

**NEVER include Co-Authored-By in checkpoint commits.**

## Rollback

If something goes wrong after the checkpoint:
```bash
git log --oneline -5    # find the checkpoint commit
git reset --soft HEAD~N  # undo N commits back to checkpoint, keep changes staged
```

NEVER use `git reset --hard` without user confirmation.

## Rules

- NEVER checkpoint code that doesn't compile
- NEVER use `git add .` or `git add -A`
- NEVER amend a checkpoint commit — create a new one
- Checkpoint messages use `checkpoint:` type prefix
- Keep checkpoint commits small and focused
- Squash checkpoint commits before PR (user's responsibility)
