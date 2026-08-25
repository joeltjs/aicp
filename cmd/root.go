package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"aicp/internal/tui"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aicp",
	Short: "Mini checkpoints for AI-made changes",
	Long: `aicp tracks AI-made changes to your working tree as lightweight
checkpoints, independent from git. Roll back anytime with aicp goto.

Run without arguments for the interactive terminal UI.`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(projectRoot())
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func projectRoot() string {
	wd, err := os.Getwd()
	cobra.CheckErr(err)
	r, err := filepath.EvalSymlinks(wd)
	cobra.CheckErr(err)
	return r
}

func confirm(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return false
	}
	v := lower(trim(sc.Text()))
	return v == "y" || v == "yes"
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
