package cmd

import (
	"fmt"

	"github.com/joeltjs/aicp/internal/ops"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a checkpoint session (captures baseline checkpoint #0)",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := projectRoot()
		m, err := ops.Start(root)
		if err != nil {
			return err
		}
		fmt.Printf("%s Checkpoint session started for %s\n", green("OK"), cyan(root))
		fmt.Printf("Baseline checkpoint #%d captured (%d files)%s\n",
			m.ID, len(m.Files), branchNote(m.Branch))
		return nil
	},
}

func branchNote(b string) string {
	if b == "" {
		return ""
	}
	return " " + dim("[branch: "+b+"]")
}

func init() {
	rootCmd.AddCommand(startCmd)
}
