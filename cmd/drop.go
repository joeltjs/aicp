package cmd

import (
	"fmt"

	"github.com/joeltjs/aicp/internal/ops"

	"github.com/spf13/cobra"
)

var dropForce bool

var dropCmd = &cobra.Command{
	Use:   "drop [id]",
	Short: "Delete the latest checkpoint (working tree is untouched)",
	RunE: func(cmd *cobra.Command, args []string) error {
		wantID := -1
		if len(args) > 0 {
			if _, err := fmt.Sscanf(args[0], "%d", &wantID); err != nil {
				return fmt.Errorf("invalid checkpoint id: %s", args[0])
			}
		}
		root := projectRoot()
		st, err := ops.StoreInfo(root)
		if err != nil {
			return err
		}
		latest, err := st.Latest()
		if err != nil {
			return err
		}
		if wantID >= 0 && wantID != latest.ID {
			return fmt.Errorf("only the latest checkpoint (#%d) can be dropped; #%d is in the middle", latest.ID, wantID)
		}
		if !dropForce {
			ok := confirm(fmt.Sprintf("Drop latest checkpoint #%d (%s)? The working tree will NOT change", latest.ID, latest.Message))
			if !ok {
				fmt.Println("Aborted.")
				return nil
			}
		}
		dropped, err := ops.DropLatest(root, wantID)
		if err != nil {
			return err
		}
		ms, err := ops.List(root)
		if err != nil {
			return err
		}
		fmt.Printf("%s Checkpoint #%d dropped. %d checkpoint(s) remain.\n", green("OK"), dropped.ID, len(ms))
		return nil
	},
}

func init() {
	dropCmd.Flags().BoolVarP(&dropForce, "force", "y", false, "skip confirmation")
	rootCmd.AddCommand(dropCmd)
}
