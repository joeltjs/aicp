package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"aicp/internal/diffutil"
	"aicp/internal/ops"
	"aicp/internal/store"
)

//go:embed static
var staticFS embed.FS

type checkpointItem struct {
	ID       int      `json:"id"`
	Time     string   `json:"time"`
	Message  string   `json:"message"`
	Branch   string   `json:"branch"`
	Auto     bool     `json:"auto"`
	Latest   bool     `json:"latest"`
	Files    int      `json:"files"`
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}

type diffFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Binary bool   `json:"binary,omitempty"`
	Patch  string `json:"patch,omitempty"`
}

type Server struct {
	st   *store.Store
	port int
}

func Serve(root string, port int) (string, error) {
	st, err := store.Load(root)
	if err != nil {
		return "", err
	}
	s := &Server{st: st, port: port}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/project", s.handleProject)
	mux.HandleFunc("GET /api/checkpoints", s.handleCheckpoints)
	mux.HandleFunc("GET /api/diff", s.handleDiff)
	mux.HandleFunc("POST /api/start", s.handleStart)
	mux.HandleFunc("POST /api/set", s.handleSet)
	mux.HandleFunc("POST /api/goto", s.handleGoto)
	mux.HandleFunc("POST /api/drop", s.handleDrop)
	mux.HandleFunc("POST /api/reset", s.handleReset)
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return "", err
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	handler := securityHeaders(hostAllowlist(csrfGuard(mux, port), port))

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}
	go func() {
		if err := http.Serve(ln, handler); err != nil && err != http.ErrServerClosed {
			fmt.Printf("aicp view server error: %v\n", err)
		}
	}()
	addr := ln.Addr().String()
	openBrowser("http://" + addr)
	return addr, nil
}

// hostAllowlist blocks DNS-rebinding requests: browsers attach the target
// Host header even to cross-site requests, so anything that is not this
// loopback server's own host:port never reaches a handler.
func hostAllowlist(next http.Handler, port int) http.Handler {
	ok := map[string]bool{
		fmt.Sprintf("127.0.0.1:%d", port): true,
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("[::1]:%d", port):     true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ok[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

// csrfGuard rejects state-changing requests whose Origin header is present
// but not one of our loopback origins. Browsers always send Origin on
// cross-site POSTs; non-browser clients (curl, scripts) omit it and pass.
func csrfGuard(next http.Handler, port int) http.Handler {
	ok := map[string]bool{
		fmt.Sprintf("http://127.0.0.1:%d", port): true,
		fmt.Sprintf("http://localhost:%d", port): true,
		fmt.Sprintf("http://[::1]:%d", port):     true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && !ok[o] {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"root": s.st.ProjectRoot,
		"size": s.st.SizeBytes(),
	})
}

func (s *Server) handleCheckpoints(w http.ResponseWriter, r *http.Request) {
	ms, err := ops.List(s.st.ProjectRoot)
	if err != nil {
		httpError(w, err)
		return
	}
	items := []checkpointItem{}
	for i, m := range ms {
		item := checkpointItem{
			ID: m.ID, Time: m.Time.Format("2006-01-02 15:04"), Message: m.Message,
			Branch: m.Branch, Auto: m.Auto, Latest: i == len(ms)-1, Files: len(m.Files),
		}
		if i > 0 {
			amd := ops.Classify(ops.ManifestMap(ms[i-1].Files), ops.ManifestMap(m.Files))
			item.Added, item.Modified, item.Deleted = amd.Added, amd.Modified, amd.Deleted
		}
		items = append(items, item)
	}
	writeJSON(w, map[string]any{"items": items})
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	aRaw := q.Get("a")
	bRaw := q.Get("b")

	var diffs []diffutil.FileDiff
	var err error

	switch {
	case bRaw == "working":
		a, perr := parseID(aRaw)
		if perr != nil {
			httpError(w, perr)
			return
		}
		diffs, err = ops.DiffWorking(s.st.ProjectRoot, a)
	default:
		a, perr := parseID(aRaw)
		if perr != nil {
			httpError(w, perr)
			return
		}
		b, perr := parseID(bRaw)
		if perr != nil {
			httpError(w, perr)
			return
		}
		if a < 0 {
			diffs, err = ops.DiffFromEmpty(s.st.ProjectRoot, b)
		} else {
			diffs, err = ops.DiffCheckpoints(s.st.ProjectRoot, a, b)
		}
	}
	if err != nil {
		httpError(w, err)
		return
	}
	files := make([]diffFile, 0, len(diffs))
	for _, d := range diffs {
		files = append(files, diffFile{Path: d.Path, Status: d.Status, Binary: d.Binary, Patch: d.Patch})
	}
	writeJSON(w, map[string]any{"files": files, "stat": diffutil.StatLine(diffs)})
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	m, err := ops.Start(s.st.ProjectRoot)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "id": m.ID, "files": len(m.Files)})
}

func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	m, amd, err := ops.Set(s.st.ProjectRoot, r.URL.Query().Get("message"))
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"ok": true, "id": m.ID, "message": m.Message,
		"added": len(amd.Added), "modified": len(amd.Modified), "deleted": len(amd.Deleted),
	})
}

func (s *Server) handleGoto(w http.ResponseWriter, r *http.Request) {
	id, perr := parseID(r.URL.Query().Get("id"))
	if perr != nil || id < 0 {
		httpError(w, fmt.Errorf("valid id required"))
		return
	}
	safety, res, target, err := ops.GotoEx(s.st.ProjectRoot, id, r.URL.Query().Get("purge") == "true")
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"ok": true, "target": target.ID, "safety": safety.ID,
		"added": res.Added, "updated": res.Updated, "deleted": res.Deleted,
	})
}

func (s *Server) handleDrop(w http.ResponseWriter, r *http.Request) {
	wantID := -1
	if raw := r.URL.Query().Get("id"); raw != "" {
		var perr error
		wantID, perr = parseID(raw)
		if perr != nil || wantID < 0 {
			httpError(w, fmt.Errorf("invalid id"))
			return
		}
	}
	dropped, err := ops.DropLatest(s.st.ProjectRoot, wantID)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "dropped": dropped.ID})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	size, err := ops.Reset(s.st.ProjectRoot)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "freed": size})
}

func parseID(raw string) (int, error) {
	if raw == "" || raw == "none" {
		return -1, nil
	}
	var id int
	if _, err := fmt.Sscanf(raw, "%d", &id); err != nil {
		return 0, fmt.Errorf("invalid id %q", raw)
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	writeJSON(w, map[string]any{"error": err.Error()})
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	} else {
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
