# Agent Instructions

## Checkpointing Discipline (aicp)

Always maintain `aicp` checkpoint discipline on any task modifying files:

1. **Before the first edit:**
   - Run `aicp status` (or `aicp start` if no session exists).
2. **After completing a unit of work:**
   - Run `aicp set -m "<imperative description>"` (max 50 characters, describe what changed).
3. **Before finishing the turn/task:**
   - Run `aicp status`. If uncheckpointed changes remain, run `aicp set -m "<description>"`.
4. **On rollback request:**
   - Run `aicp goto <id>`. Never delete history unless explicitly instructed.
