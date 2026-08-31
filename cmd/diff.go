package cmd

import (
	"fmt"
	"strings"

	"github.com/joeltjs/aicp/internal/diffutil"
	"github.com/joeltjs/aicp/internal/ops"

	"github.com/spf13/cobra"
)

const maxPatchLines = 300

var diffStatOnly bool

var diffCmd = &cobra.Command{
	Use:   "diff [a] [b]",
	Short: "Diff checkpoints or working tree (default: latest vs now)",
	RunE: func(cmd *cobra.Command, args []string) error {
		root := projectRoot()
		var ids []int
		for _, a := range args {
			var n int
			if _, err := fmt.Sscanf(a, "%d", &n); err != nil {
				return fmt.Errorf("invalid checkpoint id: %s", a)
			}
			ids = append(ids, n)
		}
		var diffs []diffutil.FileDiff
		switch len(ids) {
		case 0:
			ms, err := ops.List(root)
			if err != nil {
				return err
			}
			if len(ms) == 0 {
				return fmt.Errorf("no checkpoints found")
			}
			d, err := ops.DiffWorking(root, ms[len(ms)-1].ID)
			if err != nil {
				return err
			}
			diffs = d
			fmt.Printf("%s latest #%d <-> working tree\n", bold("diff"), ms[len(ms)-1].ID)
		case 1:
			d, err := ops.DiffWorking(root, ids[0])
			if err != nil {
				return err
			}
			diffs = d
			fmt.Printf("%s checkpoint #%d <-> working tree\n", bold("diff"), ids[0])
		case 2:
			d, err := ops.DiffCheckpoints(root, ids[0], ids[1])
			if err != nil {
				return err
			}
			diffs = d
			fmt.Printf("%s checkpoint #%d <-> checkpoint #%d\n", bold("diff"), ids[0], ids[1])
		default:
			return fmt.Errorf("usage: aicp diff [a] [b]")
		}

		if len(diffs) == 0 {
			fmt.Println(green("No differences."))
			return nil
		}
		fmt.Println(diffutil.StatLine(diffs))
		for _, d := range diffs {
			status := map[string]string{"A": "+", "M": "~", "D": "-"}[d.Status]
			colored := map[string]string{"A": green(status), "M": yellow(status), "D": red(status)}[d.Status]
			note := ""
			if d.Binary {
				note = dim(" (binary)")
			}
			fmt.Printf("\n%s %s%s\n", colored, d.Path, note)
			if diffStatOnly || d.Patch == "" {
				continue
			}
			lines := strings.Split(strings.TrimRight(d.Patch, "\n"), "\n")
			if len(lines) > maxPatchLines {
				lines = append(lines[:maxPatchLines], dim(fmt.Sprintf("... (%d more lines)", len(lines)-maxPatchLines)))
			}
			for _, l := range lines {
				printDiffLine(l)
			}
		}
		return nil
	},
}

func printDiffLine(l string) {
	switch {
	case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"):
		fmt.Println(dim(l))
	case strings.HasPrefix(l, "@@"):
		fmt.Println(diffHunk(l))
	case strings.HasPrefix(l, "+"):
		fmt.Println(diffAdd(l))
	case strings.HasPrefix(l, "-"):
		fmt.Println(diffDel(l))
	default:
		fmt.Println(dim(l))
	}
}

func init() {
	diffCmd.Flags().BoolVar(&diffStatOnly, "stat", false, "show file list only, without patches")
	rootCmd.AddCommand(diffCmd)
}
