package diffutil

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joeltjs/aicp/internal/store"
	"github.com/pmezard/go-difflib/difflib"
)

type FileDiff struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Binary bool   `json:"binary,omitempty"`
	Patch  string `json:"patch,omitempty"`
}

type Resolver func(fi store.FileInfo) ([]byte, error)

func BlobResolver(st *store.Store) Resolver {
	return func(fi store.FileInfo) ([]byte, error) {
		return st.ReadBlob(fi.Hash)
	}
}

func DiskResolver(root string) Resolver {
	return func(fi store.FileInfo) ([]byte, error) {
		p := filepathJoin(root, fi.Path)
		if fi.Kind == store.KindSymlink {
			t, err := os.Readlink(p)
			if err != nil {
				return nil, err
			}
			return []byte(t), nil
		}
		return os.ReadFile(p)
	}
}

func filepathJoin(root, slashPath string) string {
	return root + string(os.PathSeparator) + strings.ReplaceAll(slashPath, "/", string(os.PathSeparator))
}

func isBinary(b []byte) bool {
	return bytes.IndexByte(b, 0) >= 0
}

func makePatch(p string, status string, ob, nb []byte, oErr, nErr error) FileDiff {
	d := FileDiff{Path: p, Status: status}
	if oErr != nil || nErr != nil {
		d.Binary = true
		return d
	}
	if isBinary(ob) || isBinary(nb) {
		d.Binary = true
		return d
	}
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(ob)),
		B:        difflib.SplitLines(string(nb)),
		FromFile: "a/" + p,
		ToFile:   "b/" + p,
		Context:  3,
	}
	txt, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		d.Binary = true
		return d
	}
	d.Patch = txt
	return d
}

func DiffStates(oldMap, newMap map[string]store.FileInfo, oldRes, newRes Resolver) []FileDiff {
	var out []FileDiff
	paths := map[string]bool{}
	for p := range oldMap {
		paths[p] = true
	}
	for p := range newMap {
		paths[p] = true
	}
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)

	for _, p := range sorted {
		of, inOld := oldMap[p]
		nf, inNew := newMap[p]
		switch {
		case inOld && !inNew:
			ob, oerr := oldRes(of)
			out = append(out, makePatch(p, "D", ob, nil, oerr, nil))
		case !inOld && inNew:
			nb, nerr := newRes(nf)
			out = append(out, makePatch(p, "A", nil, nb, nil, nerr))
		default:
			if of.Hash == nf.Hash && of.Kind == nf.Kind && of.Mode == nf.Mode {
				continue
			}
			ob, oerr := oldRes(of)
			nb, nerr := newRes(nf)
			out = append(out, makePatch(p, "M", ob, nb, oerr, nerr))
		}
	}
	return out
}

func StatLine(diffs []FileDiff) string {
	a, m, dl, bin := 0, 0, 0, 0
	for _, d := range diffs {
		switch d.Status {
		case "A":
			a++
		case "M":
			m++
		case "D":
			dl++
		}
		if d.Binary {
			bin++
		}
	}
	s := fmt.Sprintf("%d added, %d modified, %d deleted", a, m, dl)
	if bin > 0 {
		s += fmt.Sprintf(", %d binary", bin)
	}
	return s
}
