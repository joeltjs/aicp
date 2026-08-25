package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joeltjs/aicp/internal/ignorex"
	"github.com/joeltjs/aicp/internal/store"
)

func walkTree(root string, ig *ignorex.Ignorer, fn func(rel, kind string, mode uint32, content io.Reader) error) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if ig.Match(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			tgt, lerr := os.Readlink(p)
			if lerr != nil {
				return lerr
			}
			return fn(rel, store.KindSymlink, 0, strings.NewReader(tgt))
		}
		if !d.Type().IsRegular() {
			return nil
		}
		f, oerr := os.Open(p)
		if oerr != nil {
			return oerr
		}
		defer f.Close()
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		return fn(rel, store.KindFile, uint32(info.Mode().Perm()), f)
	})
}

func Scan(root string, ig *ignorex.Ignorer) (map[string]store.FileInfo, error) {
	m := map[string]store.FileInfo{}
	err := walkTree(root, ig, func(rel, kind string, mode uint32, content io.Reader) error {
		h := sha256.New()
		if _, err := io.Copy(h, content); err != nil {
			return err
		}
		m[rel] = store.FileInfo{Path: rel, Hash: hex.EncodeToString(h.Sum(nil)), Mode: mode, Kind: kind}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

func Capture(st *store.Store, ig *ignorex.Ignorer) ([]store.FileInfo, error) {
	var out []store.FileInfo
	err := walkTree(st.ProjectRoot, ig, func(rel, kind string, mode uint32, content io.Reader) error {
		hash, _, err := st.WriteBlob(content)
		if err != nil {
			return err
		}
		out = append(out, store.FileInfo{Path: rel, Hash: hash, Mode: mode, Kind: kind})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

type ApplyResult struct {
	Added   int
	Updated int
	Deleted int
}

// clearSymlinkAncestors removes any symlink found among the parent-directory
// components of rel, so restoring a file can never traverse through a stale
// symlink and escape the project root. Manifests never contain files under
// symlinked directories (the walker does not follow symlinks), therefore a
// symlink occupying an ancestor position is always stale state to replace.
func clearSymlinkAncestors(root, rel string) error {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	cur := root
	for _, p := range parts[:len(parts)-1] {
		cur = filepath.Join(cur, p)
		fi, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return os.Remove(cur)
		}
		if !fi.IsDir() {
			return fmt.Errorf("cannot restore %s: %s is a file, not a directory", rel, cur)
		}
	}
	return nil
}

func materialize(root string, st *store.Store, fi store.FileInfo) error {
	dest := filepath.Join(root, filepath.FromSlash(fi.Path))
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := clearSymlinkAncestors(root, fi.Path); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(dest))
	if err != nil {
		return err
	}
	if realParent != root && !strings.HasPrefix(realParent, root+string(os.PathSeparator)) {
		return fmt.Errorf("restore path %s escapes project root via symlink", fi.Path)
	}
	switch fi.Kind {
	case store.KindSymlink:
		tgt, err := st.ReadBlob(fi.Hash)
		if err != nil {
			return err
		}
		return os.Symlink(string(tgt), dest)
	default:
		b, err := st.ReadBlob(fi.Hash)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, b, 0o644); err != nil {
			return err
		}
		if fi.Mode != 0 {
			return os.Chmod(dest, os.FileMode(fi.Mode).Perm())
		}
	}
	return nil
}

func pruneEmptyDirs(root string, paths []string) {
	seen := map[string]bool{}
	for _, p := range paths {
		dir := filepath.Dir(filepath.Join(root, filepath.FromSlash(p)))
		for strings.HasPrefix(dir, root) && dir != root {
			if seen[dir] {
				break
			}
			seen[dir] = true
			if err := os.Remove(dir); err != nil {
				break
			}
			dir = filepath.Dir(dir)
		}
	}
}

func Apply(st *store.Store, candidates []string, target []store.FileInfo) (ApplyResult, error) {
	root := st.ProjectRoot
	res := ApplyResult{}

	targetMap := make(map[string]store.FileInfo, len(target))
	for _, fi := range target {
		targetMap[fi.Path] = fi
	}

	for _, fi := range target {
		dest := filepath.Join(root, filepath.FromSlash(fi.Path))
		existing, err := os.Lstat(dest)
		created := err != nil || existing == nil
		if !created {
			cur, herr := hashExisting(root, fi.Path)
			if herr == nil && cur.Hash == fi.Hash && cur.Kind == fi.Kind && cur.Mode == fi.Mode {
				continue
			}
		}
		if err := materialize(root, st, fi); err != nil {
			return res, fmt.Errorf("restore %s: %w", fi.Path, err)
		}
		if created {
			res.Added++
		} else {
			res.Updated++
		}
	}

	seen := map[string]bool{}
	del := []string{}
	removeCandidate := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		if _, ok := targetMap[path]; ok {
			return
		}
		dest := filepath.Join(root, filepath.FromSlash(path))
		if _, err := os.Lstat(dest); err != nil {
			return
		}
		if err := os.RemoveAll(dest); err == nil {
			res.Deleted++
			del = append(del, path)
		}
	}
	for _, c := range candidates {
		removeCandidate(c)
	}
	pruneEmptyDirs(root, del)
	return res, nil
}

func hashExisting(root, rel string) (store.FileInfo, error) {
	dest := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Lstat(dest)
	if err != nil {
		return store.FileInfo{}, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		tgt, err := os.Readlink(dest)
		if err != nil {
			return store.FileInfo{}, err
		}
		sum := sha256.Sum256([]byte(tgt))
		return store.FileInfo{Path: rel, Hash: hex.EncodeToString(sum[:]), Mode: 0, Kind: store.KindSymlink}, nil
	}
	f, err := os.Open(dest)
	if err != nil {
		return store.FileInfo{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return store.FileInfo{}, err
	}
	return store.FileInfo{Path: rel, Hash: hex.EncodeToString(h.Sum(nil)), Mode: uint32(fi.Mode().Perm()), Kind: store.KindFile}, nil
}
