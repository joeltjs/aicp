package cmd

import (
	"fmt"
	"os"
	"strings"
)

const (
	cReset    = "\033[0m"
	cGreen    = "\033[32m"
	cYellow   = "\033[33m"
	cRed      = "\033[31m"
	cCyan     = "\033[36m"
	cDim      = "\033[2m"
	cBold     = "\033[1m"
	cBgGreen  = "\033[48;5;22m\033[38;5;120m"
	cBgRed    = "\033[48;5;52m\033[38;5;203m"
	cBgCyan   = "\033[48;5;236m\033[38;5;81m"
)

func colorEnabled() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func paint(code, s string) string {
	if !colorEnabled() || s == "" {
		return s
	}
	return code + s + cReset
}

func green(s string) string    { return paint(cGreen, s) }
func yellow(s string) string   { return paint(cYellow, s) }
func red(s string) string      { return paint(cRed, s) }
func cyan(s string) string     { return paint(cCyan, s) }
func dim(s string) string      { return paint(cDim, s) }
func bold(s string) string     { return paint(cBold, s) }
func diffAdd(s string) string  { return paint(cBgGreen, s) }
func diffDel(s string) string  { return paint(cBgRed, s) }
func diffHunk(s string) string { return paint(cBgCyan, s) }

func trim(s string) string { return strings.TrimSpace(s) }
func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func amdSummary(added, modified, deleted int) string {
	return fmt.Sprintf("%s %s %s",
		green(fmt.Sprintf("+%d", added)),
		yellow(fmt.Sprintf("~%d", modified)),
		red(fmt.Sprintf("-%d", deleted)))
}
