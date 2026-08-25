package ignorex

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

var defaults = []string{
	".git/",
	"aicp-store/",
	"node_modules/",
	"dist/",
	"build/",
	"out/",
	"coverage/",
	"__pycache__/",
	".cache/",
	".venv/",
	"venv/",
	"target/",
	".next/",
	".turbo/",
	".env",
	".env.*",
	"*.log",
	"*.tmp",
	".DS_Store",
	"Thumbs.db",
}

type Ignorer struct {
	m *gitignore.GitIgnore
}

func Load(root string) *Ignorer {
	lines := append([]string{}, defaults...)
	if b, err := os.ReadFile(filepath.Join(root, ".gitignore")); err == nil {
		lines = append(lines, strings.Split(string(b), "\n")...)
	}
	return &Ignorer{m: gitignore.CompileIgnoreLines(lines...)}
}

func (ig *Ignorer) Match(rel string, isDir bool) bool {
	if isDir {
		return ig.m.MatchesPath(rel) || ig.m.MatchesPath(rel+"/")
	}
	return ig.m.MatchesPath(rel)
}
