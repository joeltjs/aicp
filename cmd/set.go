package cmd

import (
	"fmt"
	"strings"
	"time"

	"aicp/internal/ops"

	"github.com/spf13/cobra"
)

var setMsg string

var setCmd = &cobra.Command{
	Use:   "set [message]",
	Short: "Create a checkpoint of the current working tree",
	RunE: func(cmd *cobra.Command, args []string) error {
		msg := setMsg
		if msg == "" && len(args) > 0 {
			msg = strings.Join(args, " ")
		}
		m, amd, err := ops.Set(projectRoot(), msg)
		if err != nil {
			return err
		}
		fmt.Printf("%s Checkpoint #%s saved: %s %s\n",
			green("OK"), bold(fmt.Sprintf("%d", m.ID)), cyan(m.Message),
			amdSummary(len(amd.Added), len(amd.Modified), len(amd.Deleted)))
		fmt.Printf("%s\n", dim(m.Time.Format(time.RFC3339)))
		return nil
	},
}

func init() {
	setCmd.Flags().StringVarP(&setMsg, "message", "m", "", "checkpoint message")
	rootCmd.AddCommand(setCmd)
}
