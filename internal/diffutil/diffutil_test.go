package diffutil

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeltjs/aicp/internal/store"
)

func TestDiffStatesIncludesAddedAndDeletedContent(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	write("added.txt", "brand new file\nwith two lines\n")
	write("kept.txt", "same\n")
	write("gone.txt", "this file is deleted\n")

	oldMap := map[string]store.FileInfo{
		"kept.txt": {Path: "kept.txt", Hash: hashStr("same\n"), Kind: store.KindFile},
		"gone.txt": {Path: "gone.txt", Hash: hashStr("this file is deleted\n"), Kind: store.KindFile},
	}
	newMap := map[string]store.FileInfo{
		"kept.txt":  {Path: "kept.txt", Hash: hashStr("same\n"), Kind: store.KindFile},
		"added.txt": {Path: "added.txt", Hash: hashStr("brand new file\nwith two lines\n"), Kind: store.KindFile},
	}

	res := func(fi store.FileInfo) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, fi.Path))
	}

	diffs := DiffStates(oldMap, newMap, res, res)
	byPath := map[string]FileDiff{}
	for _, d := range diffs {
		byPath[d.Path] = d
	}

	a := byPath["added.txt"]
	if a.Status != "A" || a.Patch == "" {
		t.Fatalf("added file must carry a patch: %+v", a)
	}
	if !strings.Contains(a.Patch, "+brand new file") || !strings.Contains(a.Patch, "+with two lines") {
		t.Fatalf("added patch missing content lines:\n%s", a.Patch)
	}

	dl := byPath["gone.txt"]
	if dl.Status != "D" || dl.Patch == "" {
		t.Fatalf("deleted file must carry a patch: %+v", dl)
	}
	if !strings.Contains(dl.Patch, "-this file is deleted") {
		t.Fatalf("deleted patch missing content lines:\n%s", dl.Patch)
	}

	if m := byPath["kept.txt"]; m.Status != "" && m.Patch != "" {
		t.Fatalf("unchanged file should not appear: %+v", m)
	}
}

func hashStr(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
