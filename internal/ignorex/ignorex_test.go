package ignorex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	root := t.TempDir()
	ig := Load(root)
	cases := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{".git", true, true},
		{"node_modules", true, true},
		{"node_modules/foo.js", false, true},
		{"dist", true, true},
		{".env", false, true},
		{".env.local", false, true},
		{"debug.log", false, true},
		{"src/main.go", false, false},
		{"README.md", false, false},
	}
	for _, c := range cases {
		if got := ig.Match(c.rel, c.isDir); got != c.want {
			t.Errorf("Match(%q, %v) = %v, want %v", c.rel, c.isDir, got, c.want)
		}
	}
}

func TestProjectGitignore(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("secret.txt\n*.tmp\ngenerated/\n"), 0o644)
	ig := Load(root)
	if !ig.Match("secret.txt", false) {
		t.Error("expected secret.txt ignored")
	}
	if !ig.Match("a/b.tmp", false) {
		t.Error("expected a/b.tmp ignored")
	}
	if !ig.Match("generated", true) {
		t.Error("expected generated dir ignored")
	}
	if ig.Match("main.go", false) {
		t.Error("expected main.go tracked")
	}
}
