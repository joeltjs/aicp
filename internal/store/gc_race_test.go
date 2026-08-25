package store

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestGCGracePeriod(t *testing.T) {
	t.Setenv("AICP_DATA_HOME", t.TempDir())
	root := t.TempDir()
	s, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}

	fresh := []byte("fresh blob")
	freshHash, _, err := s.WriteBlob(bytes.NewReader(fresh))
	if err != nil {
		t.Fatal(err)
	}
	old := []byte("old blob")
	oldHash, _, err := s.WriteBlob(bytes.NewReader(old))
	if err != nil {
		t.Fatal(err)
	}
	oldPath := s.BlobPath(oldHash)
	past := time.Now().Add(-20 * time.Minute)
	if err := os.Chtimes(oldPath, past, past); err != nil {
		t.Fatal(err)
	}

	removed, err := s.GC()
	if err != nil {
		t.Fatal(err)
	}
	if removed == 0 {
		t.Fatal("expected old unreferenced blob to be collected")
	}
	if _, err := s.ReadBlob(freshHash); err != nil {
		t.Fatalf("fresh in-flight blob must survive GC: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("stale blob should be gone")
	}
}
