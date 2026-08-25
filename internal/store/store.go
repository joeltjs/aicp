package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type FileInfo struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
	Mode uint32 `json:"mode"`
	Kind string `json:"kind"`
}

type Manifest struct {
	ID      int        `json:"id"`
	Time    time.Time  `json:"time"`
	Message string     `json:"message"`
	Branch  string     `json:"branch,omitempty"`
	Auto    bool       `json:"auto,omitempty"`
	Files   []FileInfo `json:"files"`
}

type Config struct {
	Root    string    `json:"root"`
	Created time.Time `json:"created"`
}

type Store struct {
	ProjectRoot string
	Dir         string
}

const KindFile = "f"
const KindSymlink = "l"

func DataHome() (string, error) {
	if x := os.Getenv("AICP_DATA_HOME"); x != "" {
		return x, nil
	}
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return x, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".local", "share"), nil
}

func ProjectID(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])[:16]
}

func StoreDirFor(root string) (string, error) {
	dh, err := DataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dh, "aicp", "projects", ProjectID(root)), nil
}

func (s *Store) ObjectsDir() string { return filepath.Join(s.Dir, "objects") }
func (s *Store) CheckpointsDir() string {
	return filepath.Join(s.Dir, "checkpoints")
}
func (s *Store) configPath() string   { return filepath.Join(s.Dir, "config.json") }
func (s *Store) BlobPath(h string) string {
	return filepath.Join(s.ObjectsDir(), h)
}
func (s *Store) manifestPath(id int) string {
	return filepath.Join(s.CheckpointsDir(), fmt.Sprintf("%04d.json", id))
}

func Init(root string) (*Store, error) {
	dir, err := StoreDirFor(root)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err == nil {
		return nil, fmt.Errorf("checkpoint session already started for this project")
	}
	for _, d := range []string{dir, filepath.Join(dir, "objects"), filepath.Join(dir, "checkpoints")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	s := &Store{ProjectRoot: root, Dir: dir}
	cfg := Config{Root: root, Created: time.Now()}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(s.configPath(), b, 0o644); err != nil {
		return nil, err
	}
	return s, nil
}

func Load(root string) (*Store, error) {
	dir, err := StoreDirFor(root)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		return nil, fmt.Errorf("no checkpoint session for this project (run: aicp start)")
	}
	return &Store{ProjectRoot: root, Dir: dir}, nil
}

func (s *Store) WriteBlob(r io.Reader) (string, bool, error) {
	tmp, err := os.CreateTemp(s.ObjectsDir(), ".tmp-*")
	if err != nil {
		return "", false, err
	}
	tmpName := tmp.Name()
	h := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(tmp, h), r)
	closeErr := tmp.Close()
	if copyErr != nil {
		os.Remove(tmpName)
		return "", false, copyErr
	}
	if closeErr != nil {
		os.Remove(tmpName)
		return "", false, closeErr
	}
	hash := hex.EncodeToString(h.Sum(nil))
	dest := s.BlobPath(hash)
	if _, err := os.Stat(dest); err == nil {
		os.Remove(tmpName)
		return hash, true, nil
	}
	if err := os.Chmod(tmpName, 0o444); err == nil {
		if err := os.Rename(tmpName, dest); err != nil {
			os.Remove(tmpName)
			return hash, false, err
		}
	} else {
		os.Remove(tmpName)
		return hash, false, err
	}
	return hash, false, nil
}

func (s *Store) ReadBlob(hash string) ([]byte, error) {
	return os.ReadFile(s.BlobPath(hash))
}

func (s *Store) SaveManifest(m *Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.CheckpointsDir(), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, s.manifestPath(m.ID))
}

func manifestIDFromName(name string) (int, bool) {
	base := strings.TrimSuffix(name, ".json")
	if len(base) != 4 {
		return 0, false
	}
	id, err := strconv.Atoi(base)
	if err != nil || id < 0 {
		return 0, false
	}
	return id, true
}

func (s *Store) Manifest(id int) (*Manifest, error) {
	p := s.manifestPath(id)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("checkpoint %d not found", id)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	m.ID = id
	return &m, nil
}

func (s *Store) Manifests() ([]*Manifest, error) {
	entries, err := os.ReadDir(s.CheckpointsDir())
	if err != nil {
		return nil, err
	}
	ids := []int{}
	for _, e := range entries {
		if id, ok := manifestIDFromName(e.Name()); ok && !strings.HasPrefix(e.Name(), ".") {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	out := make([]*Manifest, 0, len(ids))
	for _, id := range ids {
		m, err := s.Manifest(id)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *Store) Latest() (*Manifest, error) {
	ms, err := s.Manifests()
	if err != nil {
		return nil, err
	}
	if len(ms) == 0 {
		return nil, fmt.Errorf("no checkpoints found")
	}
	return ms[len(ms)-1], nil
}

func (s *Store) Drop(id int) error {
	p := s.manifestPath(id)
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("checkpoint %d not found", id)
	}
	return os.Remove(p)
}

func (s *Store) Reset() error {
	return os.RemoveAll(s.Dir)
}

func (s *Store) GC() (int, error) {
	ms, err := s.Manifests()
	if err != nil {
		return 0, err
	}
	ref := map[string]bool{}
	for _, m := range ms {
		for _, fi := range m.Files {
			ref[fi.Hash] = true
		}
	}
	entries, err := os.ReadDir(s.ObjectsDir())
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".tmp-") || !ref[name] {
			if info, err := e.Info(); err == nil && time.Since(info.ModTime()) < 15*time.Minute {
				continue
			}
			if err := os.Remove(filepath.Join(s.ObjectsDir(), name)); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}

func (s *Store) SizeBytes() int64 {
	var total int64
	filepath.WalkDir(s.Dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, ierr := d.Info(); ierr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
