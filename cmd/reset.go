package cmd

import (
	"fmt"

	"aicp/internal/ops"

	"github.com/spf13/cobra"
)

var resetForce bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete ALL checkpoints (working tree is untouched)",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := projectRoot()
		if !resetForce {
			ok := confirm(red("Delete ALL checkpoints for this project? The working tree will NOT change"))
			if !ok {
				fmt.Println("Aborted.")
				return nil
			}
		}
		size, err := ops.Reset(root)
		if err != nil {
			return err
		}
		fmt.Printf("%s All checkpoints deleted (%.1f MB freed). Working tree untouched.\n",
			green("OK"), float64(size)/(1024*1024))
		return nil
	},
}

func init() {
	resetCmd.Flags().BoolVarP(&resetForce, "force", "y", false, "skip confirmation")
	rootCmd.AddCommand(resetCmd)
}
