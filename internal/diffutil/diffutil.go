package diffutil

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"aicp/internal/store"
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
			out = append(out, FileDiff{Path: p, Status: "D"})
		case !inOld && inNew:
			out = append(out, FileDiff{Path: p, Status: "A"})
		default:
			if of.Hash == nf.Hash && of.Kind == nf.Kind && of.Mode == nf.Mode {
				continue
			}
			d := FileDiff{Path: p, Status: "M"}
			ob, oerr := oldRes(of)
			nb, nerr := newRes(nf)
			if oerr != nil || nerr != nil {
				d.Binary = true
				d.Patch = ""
				out = append(out, d)
				continue
			}
			if isBinary(ob) || isBinary(nb) {
				d.Binary = true
				out = append(out, d)
				continue
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
			} else {
				d.Patch = txt
			}
			out = append(out, d)
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
