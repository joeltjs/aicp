package cmd

import (
	"fmt"

	"github.com/joeltjs/aicp/internal/ops"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show working-tree changes vs the latest checkpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := projectRoot()
		latest, amd, err := ops.Status(root)
		if err != nil {
			return err
		}
		fmt.Printf("vs checkpoint #%s %s\n", bold(fmt.Sprintf("%d", latest.ID)), dim(latest.Message))
		if amd.Empty() {
			fmt.Println(green("Working tree matches the latest checkpoint."))
			return nil
		}
		printAMD(amd)
		return nil
	},
}

func printAMD(amd ops.AddModDel) {
	for _, p := range amd.Added {
		fmt.Printf("  %s %s\n", green("A"), p)
	}
	for _, p := range amd.Modified {
		fmt.Printf("  %s %s\n", yellow("M"), p)
	}
	for _, p := range amd.Deleted {
		fmt.Printf("  %s %s\n", red("D"), p)
	}
	fmt.Printf("\n%s\n", amdSummary(len(amd.Added), len(amd.Modified), len(amd.Deleted)))
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
