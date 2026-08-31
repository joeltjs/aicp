# aicp

Checkpoints for AI-made changes, independent from git.

When you hand your working tree to an AI agent, you lose the ability to say "go back to how it was before." `aicp` fixes that. It snapshots your tree at any moment and restores it exactly: file contents, permissions, symlinks. Your `.git` directory stays untouched, and it works in folders that are not git repositories at all.

## Install

Requires Go 1.22 or newer.

```bash
git clone https://github.com/joeltjs/aicp.git
cd ai-checkpoint
go install .
```

The `aicp` binary lands in `$GOPATH/bin` (usually `~/go/bin`). Make sure that directory is on your `PATH`.

## Quick start

```bash
cd my-project
aicp start          # capture baseline checkpoint #0

# ... let the AI loose on your code ...

aicp set -m "after auth refactor"
# ... more AI changes ...

aicp goto 1         # working tree is now identical to checkpoint #1
aicp end -y         # happy with the result? end session & wipe all checkpoints (or: aicp reset -y)
```

Run `aicp` with no arguments for the interactive terminal UI: a checkpoint table with an inline diff preview, and every operation on keys.

```text
↑/↓ select · pgup/pgdn scroll · tab mode · n set · g goto · d drop · e/R end · s start · v web · ? help · q quit
```

The same operations exist in the web dashboard: `+ set`, `start`, `drop`, `end` live above the timeline, and `goto` / `goto + purge` appear once you select a checkpoint.

## Commands

| Command | What it does |
|---|---|
| `aicp` | Interactive terminal UI: checkpoint table, inline diff preview, all operations on keys |
| `aicp start` | Begin a session and capture baseline checkpoint #0 |
| `aicp set [-m msg]` | Snapshot the current tree as a new checkpoint |
| `aicp status` | Compare the working tree against the latest checkpoint |
| `aicp list` | Show all checkpoints with per-step change summaries |
| `aicp goto N` | Restore the tree to checkpoint N. Later checkpoints stay intact, so you can move forward again |
| `aicp goto N --purge` | Restore to checkpoint N and delete every newer checkpoint (an automatic safety snapshot of the discarded state is always kept first) |
| `aicp diff [a] [b]` | Unified diff between checkpoints or against the working tree |
| `aicp drop [-y]` | Delete the latest checkpoint only. The working tree never changes |
| `aicp end [-y]` / `aicp reset [-y]` | End session and delete all checkpoints. The working tree never changes |
| `aicp view [--port N]` | Local web dashboard for browsing diffs |

## Inspecting changes

Three ways to see what changed, from quick to detailed.

**One line per checkpoint** — spot the suspicious step first:

```bash
$ aicp list
ID    WHEN                 BRANCH  TAGS          MESSAGE                    CHANGES
0     2026-08-24 21:20:04  main    latest        baseline                   -
1     2026-08-24 21:26:52  main                  rewrite login async        +1 ~2 -0
```

**File-level status vs the latest checkpoint:**

```bash
$ aicp status
vs checkpoint #1 "rewrite login async"
  A notes.txt
  M README.md
```

**Full code diff** — every added and removed line, between any two checkpoints or against your working tree:

```bash
$ aicp diff 0 1        # checkpoint 0 → checkpoint 1
$ aicp diff            # latest checkpoint → working tree right now
$ aicp diff 0 --stat   # file names only, no patch
~ src/app.js
--- a/src/app.js
+++ b/src/app.js
@@ -1,6 +1,12 @@
-function login(user, pass) {
-  if (!user || !pass) return null;
+async function login(user, pass) {
+  if (!user || !pass) throw new Error("credentials required");
...
```

The same patches appear without opening anything else: press `tab` in the terminal UI to flip the preview pane between *changes inside the selected checkpoint* and *selected checkpoint vs working tree*. In the web dashboard, pick two checkpoints in the `from → to` selectors (or choose `working`) and click a file to see its patch with line numbers.

## Web dashboard

```bash
aicp view
```

Serves a read-only dashboard on loopback and opens your browser. Set the port through an environment variable so it stays stable across runs:

```bash
echo 'AICP_DASHBOARD_PORT=3888' >> .env    # in the project you're tracking
aicp view                                   # http://127.0.0.1:3888
```

`--port` overrides both the env var and `.env`. The server refuses to start without one of them.

To keep the dashboard alive after closing the terminal:

```bash
nohup aicp view > /tmp/aicp-view.log 2>&1 &

# stop it later
pkill -f "aicp view"
```

There is no `npm run dev` and no Docker: the entire UI is embedded in the Go binary. Install once, run anywhere.

## Safety model

- Before every `goto`, aicp saves an automatic safety snapshot of the current state. Nothing can overwrite untracked work silently.
- Restores are exact. Files added after the target checkpoint are removed, deleted files come back, permissions and symlinks are preserved.
- `drop` and `reset` delete history only, never files.
- Checkpoints form a linear chain. Dropping is last-in-first-out; middle checkpoints cannot be removed.

## Storage

Snapshots live outside your project under `~/.local/share/aicp/projects/<path-hash>/`. File contents are content-addressed blobs shared between checkpoints, so each snapshot stores only what changed. Ignored paths follow your project's `.gitignore` plus built-in defaults (`node_modules`, `dist`, `.env`, logs, and similar).

## Security notes

The dashboard binds to `127.0.0.1` only, validates the `Host` header against a loopback allowlist (DNS-rebinding protection), sends a restrictive CSP, and treats every query parameter as hostile. There is no authentication by design: anyone who can reach the port already has local access. Do not tunnel it to a network.

## Automatic checkpointing with agents

`skills/auto-checkpoint/SKILL.md` teaches coding agents to checkpoint their own work: baseline before the first edit, one checkpoint per completed unit, a closing check before the session ends. No daemon, no file watcher, no switch inside aicp.

### 1. Importing the skill

Pick the row that matches your agent:

| Agent | Command |
|---|---|
| Kilo | `cp -r skills/auto-checkpoint ~/.config/kilo/skills/` |
| Claude Code | `cp -r skills/auto-checkpoint ~/.claude/skills/` |
| skills.sh ecosystem | `npx skills add joeltjs/aicp@auto-checkpoint` |
| Anything else | Point the agent at the file: *"Read and follow `skills/auto-checkpoint/SKILL.md` from this repo."* |

### 2. Enforcing agent discipline automatically

Skills teach agents *how* to checkpoint, but system prompt instructions ensure the agent *always executes* the discipline without manual prompting every time.

#### Option A: Per-Project Instruction (Recommended)
Place instructions in your project root using the standard instruction file for your agent:

| Agent / Tool | Project Instruction File |
|---|---|
| **Kilo**, **Codex**, **OpenCode** | `AGENTS.md` |
| **Claude Code** | `CLAUDE.md` |
| **GitHub Copilot**, **VS Code** | `.github/copilot-instructions.md` |
| **Cursor** | `.cursorrules` or `.cursor/rules/*.mdc` |
| **Windsurf** | `.windsurfrules` |
| **Aider**, **Pi**, **Generic Agents** | `CONVENTIONS.md` / `AGENTS.md` |

**Add this content into the instruction file:**

```markdown
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
```

#### Option B: Global Configuration (Across All Projects)
To enforce discipline globally for every project:

- **Kilo / OpenCode:**
  Add a global instruction file in `~/.config/kilo/instructions/aicp.md` or register it in `~/.config/kilo/kilo.json`:
  ```json
  {
    "instructions": ["~/.config/kilo/instructions/*.md"]
  }
  ```
- **Claude Code:**
  Add rules directly into `~/.claude/config.json` or global memory.
- **GitHub Copilot / IDEs:**
  Configure instructions in user settings under Custom Instructions.

### 3. Verifying it works

In any tracked project, ask the agent to make a small change, then run:

```bash
aicp list
```

A new checkpoint whose message describes the change means the discipline took hold. A full behavioral test looks like this:

```text
$ aicp status                      # agent runs this before its first edit
Error: no checkpoint session ...   # so it starts one:
$ aicp start
Baseline checkpoint #0 captured (3 files)

# ...agent edits src/app.js...

$ aicp set -m "throw on missing credentials"
Checkpoint #1 saved

# user reviews, dislikes the result, asks to roll back:

$ aicp goto 1
Safety snapshot #2 saved           # nothing is lost
Working tree restored to checkpoint #1

# user approves the result, so the agent pins it:

$ aicp set -m "keep state from 1"
Checkpoint #3 saved
```

Manual `aicp set` remains available for edits made outside agent sessions; manual and automatic checkpoints coexist on the same append-only timeline.

## MCP server

Agents and IDEs that speak Model Context Protocol (MCP) can drive `aicp` directly through typed tools.

Prerequisite: the binary must be installed (`go install .` or binary in `PATH`) so the agent can spawn it.

### Configuration per Agent / IDE

#### 1. Cursor
Go to **Settings** → **Features** → **MCP Servers** → **Add New MCP Server**, or add to `.cursor/mcp.json`:
```json
{
  "mcpServers": {
    "aicp": {
      "command": "aicp",
      "args": ["mcp"]
    }
  }
}
```

#### 2. Windsurf (Codeium)
Add to `~/.codeium/windsurf/mcp_config.json`:
```json
{
  "mcpServers": {
    "aicp": {
      "command": "aicp",
      "args": ["mcp"]
    }
  }
}
```

#### 3. Claude Code / Claude Desktop
- **Claude Code CLI:** Run `claude mcp add aicp aicp mcp`
- **Claude Desktop:** Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):
```json
{
  "mcpServers": {
    "aicp": {
      "command": "aicp",
      "args": ["mcp"]
    }
  }
}
```

#### 4. Kilo / OpenCode
Add to `~/.config/kilo/kilo.json` (or `.kilo/kilo.json`):
```json
{
  "mcp": {
    "aicp": {
      "type": "local",
      "command": ["aicp", "mcp"],
      "enabled": true
    }
  }
}
```

#### 5. Codex / VS Code / Other Tools
Any client supporting stdio MCP servers can spawn:
`command`: `aicp`, `args`: `["mcp"]`.

> **Note on Working Directory:** The spawned MCP server process operates on its current working directory (`cwd`), which becomes the tracked project. Launch your agent/IDE from inside the project folder.

### MCP Tools Provided

| Tool | Notes |
|---|---|
| `aicp_status`, `aicp_list`, `aicp_diff` | Read-only inspection |
| `aicp_set` | Creates a checkpoint (append-only) |
| `aicp_goto` | Restores state; optional `purge`; safety snapshot always kept |
| `aicp_drop_latest` | History only; requires explicit user request |
| `aicp_reset` | End session & delete all checkpoints; requires `confirm: true` plus an explicit user request |

### MCP Server vs Rules/Instructions: Which one is needed?

- **MCP Server** gives the AI the **tools / capabilities** (e.g. `aicp_set`, `aicp_goto`) to execute checkpoints natively without running arbitrary bash commands. The MCP server includes built-in server instructions prompting the agent to checkpoint after each unit of work.
- **Rules / Instructions (`AGENTS.md`, `.cursorrules`, `CLAUDE.md`)** enforce the **strict habit / timing** (telling the agent *never* to forget checkpointing before the first edit and after completing a task).

Using both guarantees maximum discipline and reliability across any AI tool.

### Debugging the server

Prove the binary answers before blaming the agent:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}}' | aicp mcp
```

A single JSON line containing `"result"` means the server is alive. For point-and-click testing, run the official inspector:

```bash
npx @modelcontextprotocol/inspector aicp mcp
```

It opens a page where you list tools and call them one by one. Common failures:

| Symptom | Cause and fix |
|---|---|
| `Error: no checkpoint session for this project` | Normal before tracking starts. Run `aicp start` once, or call it through MCP if you added that flow yourself |
| Agent lists tools but every call fails | The process cwd is not the tracked project. Launch the agent from the project folder |
| Server exits immediately | Binary not found on PATH. Run `go install .` again and reopen the agent |

Skill or MCP? The skill teaches *when* to checkpoint; MCP provides *typed access* for agents without shell access. They compose fine.

## License

MIT.
