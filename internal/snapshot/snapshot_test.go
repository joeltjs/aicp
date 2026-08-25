package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"aicp/internal/ignorex"
	"aicp/internal/store"
)

func setup(t *testing.T) (*store.Store, string) {
	t.Helper()
	t.Setenv("AICP_DATA_HOME", t.TempDir())
	root := t.TempDir()
	st, err := store.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	return st, root
}

func writeFile(t *testing.T, root, rel, content string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(root, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureIgnoresAndDedups(t *testing.T) {
	st, root := setup(t)
	writeFile(t, root, "a.txt", "hello", 0o644)
	writeFile(t, root, "node_modules/x.js", "junk", 0o644)
	files, err := Capture(st, ignorex.Load(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "a.txt" {
		t.Fatalf("unexpected capture: %+v", files)
	}
	if _, err := st.ReadBlob(files[0].Hash); err != nil {
		t.Fatalf("blob missing: %v", err)
	}
}

func TestApplyRestoresTree(t *testing.T) {
	st, root := setup(t)
	ig := ignorex.Load(root)
	writeFile(t, root, "keep.txt", "same", 0o644)
	writeFile(t, root, "sub/mod.txt", "old", 0o644)
	writeFile(t, root, "del.txt", "bye", 0o644)
	base, err := Capture(st, ig)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, root, "mod.txt", "new content", 0o644)
	writeFile(t, root, "sub/mod.txt", "changed", 0o644)
	os.Remove(filepath.Join(root, "del.txt"))
	writeFile(t, root, "added.txt", "brand new", 0o600)

	candidates := []string{}
	scanMap, err := Scan(root, ig)
	if err != nil {
		t.Fatal(err)
	}
	for p := range scanMap {
		candidates = append(candidates, p)
	}

	res, err := Apply(st, candidates, base)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 || res.Deleted != 2 || res.Updated != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}

	after, err := Scan(root, ig)
	if err != nil {
		t.Fatal(err)
	}
	baseMap := map[string]store.FileInfo{}
	for _, fi := range base {
		baseMap[fi.Path] = fi
	}
	for p, fi := range baseMap {
		got, ok := after[p]
		if !ok {
			t.Errorf("%s missing after restore", p)
			continue
		}
		if got.Hash != fi.Hash || got.Mode != fi.Mode || got.Kind != fi.Kind {
			t.Errorf("%s mismatch: got %+v want %+v", p, got, fi)
		}
	}
	for p := range after {
		if _, ok := baseMap[p]; !ok {
			t.Errorf("%s should have been deleted by restore", p)
		}
	}
}

func TestSymlinkHandling(t *testing.T) {
	st, root := setup(t)
	ig := ignorex.Load(root)
	writeFile(t, root, "real.txt", "target", 0o644)
	os.Symlink(filepath.Join(root, "real.txt"), filepath.Join(root, "link.txt"))
	base, err := Capture(st, ig)
	if err != nil {
		t.Fatal(err)
	}
	var link *store.FileInfo
	for i := range base {
		if base[i].Path == "link.txt" {
			link = &base[i]
		}
	}
	if link == nil || link.Kind != store.KindSymlink {
		t.Fatalf("symlink not captured: %+v", base)
	}

	os.Remove(filepath.Join(root, "link.txt"))
	res, err := Apply(st, []string{}, base)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 1 {
		t.Fatalf("expected symlink re-added, got %+v", res)
	}
	tgt, err := os.Readlink(filepath.Join(root, "link.txt"))
	if err != nil || tgt != filepath.Join(root, "real.txt") {
		t.Fatalf("symlink not restored correctly: %q %v", tgt, err)
	}
}

func TestRestoreReplacesDirSymlinkWithoutEscape(t *testing.T) {
	st, root := setup(t)
	ig := ignorex.Load(root)

	outside := t.TempDir()
	writeFile(t, outside, "victim.txt", "original", 0o644)

	writeFile(t, root, "link/hacked.txt", "hacked!", 0o644)
	base, err := Capture(st, ig)
	if err != nil {
		t.Fatal(err)
	}

	os.RemoveAll(filepath.Join(root, "link"))
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(st, []string{}, base); err != nil {
		t.Fatalf("apply should succeed by replacing stale symlink: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(root, "link", "hacked.txt"))
	if err != nil || string(b) != "hacked!" {
		t.Fatalf("file not restored inside project: %q %v", b, err)
	}
	vb, err := os.ReadFile(filepath.Join(outside, "victim.txt"))
	if err != nil || string(vb) != "original" {
		t.Fatalf("file outside project was overwritten: %q %v", vb, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "hacked.txt")); !os.IsNotExist(err) {
		t.Fatal("arbitrary write escaped the project root")
	}
}
