package cmd

import (
	"fmt"

	"github.com/joeltjs/aicp/internal/ops"

	"github.com/spf13/cobra"
)

var gotoPurge bool

var gotoCmd = &cobra.Command{
	Use:   "goto <id>",
	Short: "Restore the working tree to a checkpoint (--purge also deletes newer ones)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var id int
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return fmt.Errorf("invalid checkpoint id: %s", args[0])
		}
		root := projectRoot()
		safety, res, target, err := ops.GotoEx(root, id, gotoPurge)
		if err != nil {
			return err
		}
		fmt.Printf("%s Safety snapshot #%d saved (drop it with: aicp drop)\n", green("OK"), safety.ID)
		fmt.Printf("Working tree restored to checkpoint #%s %s %s\n",
			bold(fmt.Sprintf("%d", target.ID)),
			cyan(target.Message),
			green(fmt.Sprintf("+%d added", res.Added))+" "+
				yellow(fmt.Sprintf("%d updated", res.Updated))+" "+
				red(fmt.Sprintf("%d deleted", res.Deleted)))
		if gotoPurge {
			fmt.Printf("%s All checkpoints newer than #%d were purged (safety #%d kept).\n",
				dim("Note:"), id, safety.ID)
		} else {
			fmt.Printf("%s Checkpoints after #%d are kept - go forward again anytime.\n",
				dim("Note:"), id)
		}
		return nil
	},
}

func init() {
	gotoCmd.Flags().BoolVar(&gotoPurge, "purge", false, "delete all checkpoints newer than the target (safety snapshot is kept)")
	rootCmd.AddCommand(gotoCmd)
}
