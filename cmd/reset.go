package cmd

import (
	"fmt"

	"github.com/joeltjs/aicp/internal/ops"

	"github.com/spf13/cobra"
)

var resetForce bool

var resetCmd = &cobra.Command{
	Use:     "reset",
	Aliases: []string{"end"},
	Short:   "End session & delete ALL checkpoints (working tree is untouched)",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := projectRoot()
		if !resetForce {
			ok := confirm(red("End checkpoint session and delete all checkpoints? The working tree will NOT change"))
			if !ok {
				fmt.Println("Aborted.")
				return nil
			}
		}
		size, err := ops.Reset(root)
		if err != nil {
			return err
		}
		fmt.Printf("%s Checkpoint session ended. All checkpoints deleted (%.1f MB freed). Working tree untouched.\n",
			green("OK"), float64(size)/(1024*1024))
		return nil
	},
}

func init() {
	resetCmd.Flags().BoolVarP(&resetForce, "force", "y", false, "skip confirmation")
	rootCmd.AddCommand(resetCmd)
}
