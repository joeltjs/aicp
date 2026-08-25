package cmd

import (
	"github.com/joeltjs/aicp/internal/mcpserver"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the MCP server over stdio (for AI agents)",
	Long: `Expose aicp as Model Context Protocol tools so AI agents can list,
diff, set, goto and drop checkpoints without shell access.

Configure your agent with:
  {"command": "aicp", "args": ["mcp"]}
The working directory of that process is the tracked project.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpserver.Run(projectRoot())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
