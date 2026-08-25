package cmd

import (
	"fmt"
	"strings"
	"time"

	"aicp/internal/ops"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all checkpoints with change summaries",
	RunE: func(cmd *cobra.Command, args []string) error {
		ms, err := ops.List(projectRoot())
		if err != nil {
			return err
		}
		if len(ms) == 0 {
			fmt.Println("No checkpoints (run: aicp start)")
			return nil
		}
		latestID := ms[len(ms)-1].ID
		fmt.Printf("%-5s %-20s %-14s %-14s %-26s %s\n", "ID", "WHEN", "BRANCH", "TAGS", "MESSAGE", "CHANGES")
		for i, m := range ms {
			tags := []string{}
			if m.Auto {
				tags = append(tags, yellow("auto"))
			}
			if m.ID == latestID {
				tags = append(tags, cyan("latest"))
			}
			tagStr := strings.Join(tags, ",")
			if tagStr == "" {
				tagStr = "-"
			}
			changes := dim("-")
			if i > 0 {
				amd := ops.Classify(ops.ManifestMap(ms[i-1].Files), ops.ManifestMap(m.Files))
				changes = amdSummary(len(amd.Added), len(amd.Modified), len(amd.Deleted))
			}
			msg := m.Message
			if len(msg) > 26 {
				msg = msg[:23] + "..."
			}
			fmt.Printf("%-5d %-20s %-14s %-14s %-26s %s\n",
				m.ID,
				m.Time.Format(time.RFC3339)[:19],
				trunc(branchOrNone(m.Branch), 14),
				trunc(tagStr, 14),
				msg,
				changes)
		}
		return nil
	},
}

func branchOrNone(b string) string {
	if b == "" {
		return "-"
	}
	return b
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func init() {
	rootCmd.AddCommand(listCmd)
}
