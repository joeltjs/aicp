package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeltjs/aicp/internal/store"
)

func testEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("AICP_DATA_HOME", t.TempDir())
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func wf(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte(content), 0o644)
}

func TestFullWorkflow(t *testing.T) {
	root := testEnv(t)

	wf(t, root, "main.go", "package main")
	wf(t, root, "README.md", "hello")
	m0, err := Start(root)
	if err != nil {
		t.Fatal(err)
	}
	if m0.ID != 0 || len(m0.Files) != 2 {
		t.Fatalf("baseline wrong: id=%d files=%d", m0.ID, len(m0.Files))
	}

	wf(t, root, "auth.go", "auth logic v1")
	wf(t, root, "main.go", "package main // edited by AI")
	m1, amd1, err := Set(root, "add auth")
	if err != nil {
		t.Fatal(err)
	}
	if m1.ID != 1 || len(amd1.Added) != 1 || len(amd1.Modified) != 1 {
		t.Fatalf("set1 wrong: %+v %+v", m1, amd1)
	}

	wf(t, root, "auth.go", "auth logic v2 broken")
	wf(t, root, "junk_ai.txt", "ai garbage")
	m2, _, err := Set(root, "break things")
	if err != nil {
		t.Fatal(err)
	}
	if m2.ID != 2 {
		t.Fatalf("expected id 2, got %d", m2.ID)
	}

	latest, amd, err := Status(root)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != 2 || !amd.Empty() {
		t.Fatalf("status should be clean: #%d %+v", latest.ID, amd)
	}

	safety, res, target, err := GotoEx(root, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if safety == nil || !safety.Auto || target.ID != 1 {
		t.Fatalf("goto meta wrong: safety=%+v target=%+v", safety, target)
	}
	b, _ := os.ReadFile(filepath.Join(root, "auth.go"))
	if string(b) != "auth logic v1" {
		t.Fatalf("auth.go not restored: %q", b)
	}
	if _, err := os.Stat(filepath.Join(root, "junk_ai.txt")); !os.IsNotExist(err) {
		t.Fatal("junk_ai.txt should have been deleted by goto")
	}
	b, _ = os.ReadFile(filepath.Join(root, "main.go"))
	if string(b) != "package main // edited by AI" {
		t.Fatalf("main.go should be CP#1 state: %q (res=%+v)", b, res)
	}

	_, amdNow, err := Status(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range amdNow.Deleted {
		if p == "junk_ai.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("status vs new latest should show junk_ai.txt deleted: %+v", amdNow)
	}

	ms, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 3 {
		t.Fatalf("purge should leave only #0, #1 and safety #%d, got %d manifests", safety.ID, len(ms))
	}
	for _, m := range ms {
		if m.ID != 0 && m.ID != 1 && m.ID != safety.ID {
			t.Fatalf("unexpected manifest #%d after purge", m.ID)
		}
	}

	diffs, err := DiffCheckpoints(root, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, d := range diffs {
		paths[d.Path] = true
	}
	if !paths["auth.go"] {
		t.Fatalf("cp diffs missing expected paths: %+v", diffs)
	}

	dw, err := DiffWorking(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	statPaths := map[string]string{}
	for _, d := range dw {
		statPaths[d.Path] = d.Status
	}
	if statPaths["README.md"] != "" || statPaths["main.go"] != "M" {
		t.Fatalf("diff working unexpected: %+v", dw)
	}

	if _, err := DropLatest(root, 1); err == nil {
		t.Fatal("dropping middle checkpoint must fail")
	}
	dropped, err := DropLatest(root, -1)
	if err != nil {
		t.Fatal(err)
	}
	if dropped.ID != 3 {
		t.Fatalf("expected drop of safety #3, got #%d", dropped.ID)
	}
	ms, _ = List(root)
	if len(ms) != 2 {
		t.Fatalf("expected 2 after drop, got %d", len(ms))
	}
	st, err := store.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	nGC, err := st.GC()
	if err != nil {
		t.Fatal(err)
	}
	_ = nGC

	size, err := Reset(root)
	if err != nil || size <= 0 {
		t.Fatalf("reset failed: %v %d", err, size)
	}
	if _, err := store.Load(root); err == nil {
		t.Fatal("store should be gone after reset")
	}
	if _, err := os.Stat(filepath.Join(root, "main.go")); err != nil {
		t.Fatal("working tree must survive reset")
	}
}

func TestSetWithoutStartFails(t *testing.T) {
	root := testEnv(t)
	_, _, err := Set(root, "nope")
	if err == nil || !strings.Contains(err.Error(), "aicp start") {
		t.Fatalf("expected hint error, got %v", err)
	}
}
