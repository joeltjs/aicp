---
name: auto-checkpoint
description: Checkpointing discipline for AI coding agents using aicp. Use when editing files in any project tracked by aicp, or when the user mentions checkpoints, aicp, or asks you to checkpoint your changes automatically. Ensures every completed unit of AI work is snapshotted so the user can roll back.
---

# Auto-checkpoint with aicp

You are working in a project that uses `aicp` for rollback safety. Follow these rules on every task that touches files.

Checkpoints are append-only. Nothing you do with `set` or `goto` destroys anything: `goto` always saves an automatic safety snapshot of the current state first, and `set` only appends a new entry. Manual actions by the user run against the same rules, so you can never overwrite their work either.

## 1. Before your first edit

Run:

```bash
aicp status
```

- If it fails with "no checkpoint session", run `aicp start`. This captures baseline #0 of the untouched tree.
- If it succeeds, note the latest checkpoint id from its output.

## 2. After completing a unit of work

The moment one coherent unit is done (a bug fix, one feature, one refactor step), create a checkpoint:

```bash
aicp set -m "fix login redirect loop"
```

Rules for messages:
- Max 50 characters, imperative mood, describe what changed.
- Never mention yourself, the word "checkpoint", or the date.

One unit equals one checkpoint. Do not batch unrelated changes into one checkpoint, and do not checkpoint while files are half-written.

## 3. Before ending your turn or the chat session

Run `aicp status`. If the tree differs from the latest checkpoint, run one final `aicp set -m "..."`. The user reviews your work in the dashboard afterwards; an uncheckpointed tail would be invisible there.

## 4. Rollback requests

When the user says a recent change is wrong ("balikin ke checkpoint 3", "rollback", "undo"):

```bash
aicp goto <id>
```

Then report in this exact shape:

```
Restored to #3 "add auth middleware". Safety snapshot of previous state saved as #6.
```

If the user confirms the restored state is good ("looks good", "keep this"), pin it so it becomes the newest point in history:

```bash
aicp set -m "keep state from #3"
```

Do not pin automatically on every rollback; only after explicit confirmation.

## 5. Cleaning up / ending session

```bash
aicp end      # or: aicp reset (deletes all checkpoints)
aicp drop     # deletes the latest checkpoint
```

These erase history. If the user asks to clean up, confirm once, then run them exactly as asked and report what was freed.
