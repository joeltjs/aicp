package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/joeltjs/aicp/internal/diffutil"
	"github.com/joeltjs/aicp/internal/ops"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func textResult(s string) *mcp.CallToolResult {
	return mcp.NewToolResultText(s)
}

func errResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

func argsMap(req mcp.CallToolRequest) map[string]any {
	if m, ok := req.Params.Arguments.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func formatDiffsPlain(diffs []diffutil.FileDiff) string {
	var sb strings.Builder
	for _, d := range diffs {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", d.Status, d.Path))
		if d.Patch != "" {
			sb.WriteString(d.Patch)
			if !strings.HasSuffix(d.Patch, "\n") {
				sb.WriteString("\n")
			}
		}
	}
	if sb.Len() == 0 {
		return "(no differences)"
	}
	return sb.String()
}

func argInt(args map[string]any, key string) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("missing required param: %s", key)
	}
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	default:
		return 0, fmt.Errorf("param %s must be a number", key)
	}
}

func argBool(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func argString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Run serves the aicp MCP server over stdio. root is the project directory
// the server operates on (normally the process working directory).
func Run(root string) error {
	s := server.NewMCPServer(
		"aicp",
		"0.1.0",
		server.WithInstructions("Checkpoint tools for AI-made changes. Create a checkpoint with 'set' after each completed unit of work. 'goto' restores an exact state and always saves an automatic safety snapshot first. Never call drop/reset unless the user explicitly asked."),
	)

	statusTool := mcp.NewTool("aicp_status",
		mcp.WithDescription("Show working-tree changes vs the latest checkpoint"),
	)
	s.AddTool(statusTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		latest, amd, err := ops.Status(root)
		if err != nil {
			return errResult(err), nil
		}
		if amd.Empty() {
			return textResult(fmt.Sprintf("working tree matches checkpoint #%d (%s)", latest.ID, latest.Message)), nil
		}
		return textResult(fmt.Sprintf("vs #%d %s\nadded:    %s\nmodified: %s\ndeleted:  %s",
			latest.ID, latest.Message,
			strings.Join(amd.Added, ", "), strings.Join(amd.Modified, ", "), strings.Join(amd.Deleted, ", "))), nil
	})

	listTool := mcp.NewTool("aicp_list",
		mcp.WithDescription("List all checkpoints with per-step change summaries"),
	)
	s.AddTool(listTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ms, err := ops.List(root)
		if err != nil {
			return errResult(err), nil
		}
		var sb strings.Builder
		for i, m := range ms {
			changes := "-"
			if i > 0 {
				amd := ops.Classify(ops.ManifestMap(ms[i-1].Files), ops.ManifestMap(m.Files))
				changes = fmt.Sprintf("+%d ~%d -%d", len(amd.Added), len(amd.Modified), len(amd.Deleted))
			}
			tags := ""
			if m.Auto {
				tags += " [auto]"
			}
			if i == len(ms)-1 {
				tags += " [latest]"
			}
			fmt.Fprintf(&sb, "#%d %s [%s]%s %s (%d files) changes: %s\n",
				m.ID, m.Time.Format("2006-01-02 15:04"), m.Branch, tags, m.Message, len(m.Files), changes)
		}
		return textResult(sb.String()), nil
	})

	diffTool := mcp.NewTool("aicp_diff",
		mcp.WithDescription("Unified code diff between two checkpoints, or one checkpoint vs the current working tree"),
		mcp.WithNumber("a", mcp.Description("source checkpoint id")),
		mcp.WithNumber("b", mcp.Description("target checkpoint id; omit when working=true")),
		mcp.WithBoolean("working", mcp.Description("if true, diff checkpoint a against the working tree")),
	)
	s.AddTool(diffTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := argsMap(req)
		a, err := argInt(args, "a")
		if err != nil {
			return errResult(err), nil
		}
		var diffs []diffutil.FileDiff
		if argBool(args, "working") {
			diffs, err = ops.DiffWorking(root, a)
		} else {
			b, berr := argInt(args, "b")
			if berr != nil {
				return errResult(berr), nil
			}
			diffs, err = ops.DiffCheckpoints(root, a, b)
		}
		if err != nil {
			return errResult(err), nil
		}
		out := diffutil.StatLine(diffs) + "\n\n" + formatDiffsPlain(diffs)
		if len(out) > 60000 {
			out = out[:60000] + "\n...(truncated)"
		}
		return textResult(out), nil
	})

	setTool := mcp.NewTool("aicp_set",
		mcp.WithDescription("Create a new checkpoint from the current working tree. Call after each completed unit of work"),
		mcp.WithString("message", mcp.Description("short imperative message, max 50 chars")),
	)
	s.AddTool(setTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m, amd, err := ops.Set(root, argString(argsMap(req), "message"))
		if err != nil {
			return errResult(err), nil
		}
		return textResult(fmt.Sprintf("checkpoint #%d saved: %s (+%d ~%d -%d)",
			m.ID, m.Message, len(amd.Added), len(amd.Modified), len(amd.Deleted))), nil
	})

	gotoTool := mcp.NewTool("aicp_goto",
		mcp.WithDescription("Restore the working tree to a checkpoint. An automatic safety snapshot is always created first"),
		mcp.WithNumber("id", mcp.Description("target checkpoint id"), mcp.Required()),
		mcp.WithBoolean("purge", mcp.Description("also delete every checkpoint newer than id (safety snapshot is kept)")),
	)
	s.AddTool(gotoTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := argInt(argsMap(req), "id")
		if err != nil {
			return errResult(err), nil
		}
		safety, res, target, gerr := ops.GotoEx(root, id, argBool(argsMap(req), "purge"))
		if gerr != nil {
			return errResult(gerr), nil
		}
		return textResult(fmt.Sprintf("restored to #%d (+%d ~%d -%d). safety snapshot #%d saved",
			target.ID, res.Added, res.Updated, res.Deleted, safety.ID)), nil
	})

	dropTool := mcp.NewTool("aicp_drop_latest",
		mcp.WithDescription("Delete ONLY the latest checkpoint (history only; files untouched). Requires explicit user request"),
	)
	s.AddTool(dropTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dropped, err := ops.DropLatest(root, -1)
		if err != nil {
			return errResult(err), nil
		}
		return textResult(fmt.Sprintf("checkpoint #%d dropped", dropped.ID)), nil
	})

	resetTool := mcp.NewTool("aicp_reset",
		mcp.WithDescription("Delete ALL checkpoints (files untouched). Destructive: requires confirm=true and an explicit user request"),
		mcp.WithBoolean("confirm", mcp.Description("must be true"), mcp.Required()),
	)
	s.AddTool(resetTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if !argBool(argsMap(req), "confirm") {
			return errResult(fmt.Errorf("reset refused: pass confirm=true and make sure the user explicitly asked")), nil
		}
		size, err := ops.Reset(root)
		if err != nil {
			return errResult(err), nil
		}
		return textResult(fmt.Sprintf("all checkpoints deleted (%.1f KB freed)", float64(size)/1024)), nil
	})

	return server.ServeStdio(s)
}
