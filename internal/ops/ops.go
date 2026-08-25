package ops

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/joeltjs/aicp/internal/diffutil"
	"github.com/joeltjs/aicp/internal/ignorex"
	"github.com/joeltjs/aicp/internal/snapshot"
	"github.com/joeltjs/aicp/internal/store"
)

type AddModDel struct {
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}

func (a AddModDel) Empty() bool {
	return len(a.Added) == 0 && len(a.Modified) == 0 && len(a.Deleted) == 0
}

func CurrentBranch(root string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func Start(root string) (*store.Manifest, error) {
	st, err := store.Init(root)
	if err != nil {
		return nil, err
	}
	files, err := snapshot.Capture(st, ignorex.Load(root))
	if err != nil {
		return nil, err
	}
	m := &store.Manifest{
		ID:      0,
		Time:    time.Now(),
		Message: "baseline",
		Branch:  CurrentBranch(root),
		Files:   files,
	}
	if err := st.SaveManifest(m); err != nil {
		return nil, err
	}
	return m, nil
}

func Set(root, msg string) (*store.Manifest, AddModDel, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, AddModDel{}, err
	}
	ms, err := st.Manifests()
	if err != nil {
		return nil, AddModDel{}, err
	}
	if len(ms) == 0 {
		return nil, AddModDel{}, fmt.Errorf("no checkpoints found (run: aicp start)")
	}
	prev := ms[len(ms)-1]
	files, err := snapshot.Capture(st, ignorex.Load(root))
	if err != nil {
		return nil, AddModDel{}, err
	}
	if strings.TrimSpace(msg) == "" {
		msg = fmt.Sprintf("checkpoint %d", prev.ID+1)
	}
	m := &store.Manifest{
		ID:      prev.ID + 1,
		Time:    time.Now(),
		Message: msg,
		Branch:  CurrentBranch(root),
		Files:   files,
	}
	if err := st.SaveManifest(m); err != nil {
		return nil, AddModDel{}, err
	}
	amd := Classify(manifestMap(prev.Files), manifestMap(files))
	return m, amd, nil
}

func manifestMap(files []store.FileInfo) map[string]store.FileInfo {
	return ManifestMap(files)
}

func ManifestMap(files []store.FileInfo) map[string]store.FileInfo {
	m := make(map[string]store.FileInfo, len(files))
	for _, fi := range files {
		m[fi.Path] = fi
	}
	return m
}

func Classify(oldMap, newMap map[string]store.FileInfo) AddModDel {
	var amd AddModDel
	for p, of := range oldMap {
		nf, ok := newMap[p]
		if !ok {
			amd.Deleted = append(amd.Deleted, p)
			continue
		}
		if of.Hash != nf.Hash || of.Kind != nf.Kind || of.Mode != nf.Mode {
			amd.Modified = append(amd.Modified, p)
		}
	}
	for p := range newMap {
		if _, ok := oldMap[p]; !ok {
			amd.Added = append(amd.Added, p)
		}
	}
	sort.Strings(amd.Added)
	sort.Strings(amd.Modified)
	sort.Strings(amd.Deleted)
	return amd
}

func Status(root string) (*store.Manifest, AddModDel, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, AddModDel{}, err
	}
	latest, err := st.Latest()
	if err != nil {
		return nil, AddModDel{}, err
	}
	cur, err := snapshot.Scan(root, ignorex.Load(root))
	if err != nil {
		return nil, AddModDel{}, err
	}
	return latest, Classify(manifestMap(latest.Files), cur), nil
}

func List(root string) ([]*store.Manifest, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, err
	}
	return st.Manifests()
}

func Goto(root string, id int) (*store.Manifest, snapshot.ApplyResult, *store.Manifest, error) {
	return GotoEx(root, id, false)
}

// GotoEx restores the tree to checkpoint id. With purge=true, every
// checkpoint newer than id is deleted afterwards. The automatic safety
// snapshot is always kept, so the discarded state remains recoverable until
// the user drops or resets it explicitly.
func GotoEx(root string, id int, purge bool) (*store.Manifest, snapshot.ApplyResult, *store.Manifest, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, snapshot.ApplyResult{}, nil, err
	}
	target, err := st.Manifest(id)
	if err != nil {
		return nil, snapshot.ApplyResult{}, nil, err
	}
	latest, err := st.Latest()
	if err != nil {
		return nil, snapshot.ApplyResult{}, nil, err
	}
	safetyFiles, err := snapshot.Capture(st, ignorex.Load(root))
	if err != nil {
		return nil, snapshot.ApplyResult{}, nil, err
	}
	safety := &store.Manifest{
		ID:      latest.ID + 1,
		Time:    time.Now(),
		Message: fmt.Sprintf("auto safety before goto #%d", id),
		Branch:  CurrentBranch(root),
		Auto:    true,
		Files:   safetyFiles,
	}
	if err := st.SaveManifest(safety); err != nil {
		return nil, snapshot.ApplyResult{}, nil, err
	}

	candidates := map[string]bool{}
	for _, fi := range latest.Files {
		candidates[fi.Path] = true
	}
	curScan, err := snapshot.Scan(root, ignorex.Load(root))
	if err != nil {
		return nil, snapshot.ApplyResult{}, nil, err
	}
	for p := range curScan {
		candidates[p] = true
	}
	candList := make([]string, 0, len(candidates))
	for p := range candidates {
		candList = append(candList, p)
	}
	sort.Strings(candList)

	res, err := snapshot.Apply(st, candList, target.Files)
	if err != nil {
		return safety, res, target, err
	}

	purged := 0
	if purge {
		ms, lerr := st.Manifests()
		if lerr == nil {
			for _, m := range ms {
				if m.ID > id && m.ID != safety.ID {
					st.Drop(m.ID)
					purged++
				}
			}
			st.GC()
		}
	}
	_ = purged
	return safety, res, target, nil
}

func DiffCheckpoints(root string, a, b int) ([]diffutil.FileDiff, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, err
	}
	ma, err := st.Manifest(a)
	if err != nil {
		return nil, err
	}
	mb, err := st.Manifest(b)
	if err != nil {
		return nil, err
	}
	res := diffutil.BlobResolver(st)
	return diffutil.DiffStates(manifestMap(ma.Files), manifestMap(mb.Files), res, res), nil
}

func DiffWorking(root string, a int) ([]diffutil.FileDiff, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, err
	}
	ma, err := st.Manifest(a)
	if err != nil {
		return nil, err
	}
	cur, err := snapshot.Scan(root, ignorex.Load(root))
	if err != nil {
		return nil, err
	}
	return diffutil.DiffStates(manifestMap(ma.Files), cur, diffutil.BlobResolver(st), diffutil.DiskResolver(root)), nil
}

func DiffFromEmpty(root string, b int) ([]diffutil.FileDiff, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, err
	}
	mb, err := st.Manifest(b)
	if err != nil {
		return nil, err
	}
	res := diffutil.BlobResolver(st)
	return diffutil.DiffStates(map[string]store.FileInfo{}, manifestMap(mb.Files), res, res), nil
}

func DropLatest(root string, wantID int) (*store.Manifest, error) {
	st, err := store.Load(root)
	if err != nil {
		return nil, err
	}
	latest, err := st.Latest()
	if err != nil {
		return nil, err
	}
	if wantID >= 0 && wantID != latest.ID {
		return nil, fmt.Errorf("only the latest checkpoint (#%d) can be dropped; #%d is in the middle", latest.ID, wantID)
	}
	if err := st.Drop(latest.ID); err != nil {
		return nil, err
	}
	st.GC()
	return latest, nil
}

func Reset(root string) (int64, error) {
	st, err := store.Load(root)
	if err != nil {
		return 0, err
	}
	size := st.SizeBytes()
	if err := st.Reset(); err != nil {
		return size, err
	}
	return size, nil
}

func StoreInfo(root string) (*store.Store, error) {
	return store.Load(root)
}
