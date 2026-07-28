// exec-agent is the executor-side counterpart of pkg/executor.ExecAgentClient.
//
// It runs inside a long-lived tools container/pod (e.g. Kali) and exposes a
// small HTTP API that PentAGI calls instead of talking to a Docker daemon:
//
//	GET    /v1/health              liveness
//	POST   /v1/workdir  {flow_id}  create /work/flow-<id>
//	DELETE /v1/workdir?flow_id=N   remove /work/flow-<id>
//	POST   /v1/exec {cmd,workdir}  run a command, stream raw combined stdout+stderr
//	GET    /v1/stat?path=          stat a path (container.PathStat JSON)
//	GET    /v1/ls?path=            list a directory ([]container.PathStat JSON)
//	PUT    /v1/fs?path=            extract a tar (request body) into path
//	GET    /v1/fs?path=            return a tar of path
//
// Auth: a static bearer token (EXECUTOR_TOKEN). All filesystem access is confined
// to WORK_ROOT (default /work). This process needs no privileges beyond running
// the pentest tools baked into its image.
package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
)

var (
	workRoot   = envOr("WORK_ROOT", "/work")
	token      = os.Getenv("EXECUTOR_TOKEN")
	listenAddr = envOr("EXECUTOR_LISTEN", ":8022")
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	if err := os.MkdirAll(workRoot, 0o755); err != nil {
		log.Fatalf("cannot create work root %s: %v", workRoot, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", handleHealth)
	mux.HandleFunc("/v1/workdir", auth(handleWorkdir))
	mux.HandleFunc("/v1/exec", auth(handleExec))
	mux.HandleFunc("/v1/stat", auth(handleStat))
	mux.HandleFunc("/v1/ls", auth(handleLs))
	mux.HandleFunc("/v1/fs", auth(handleFS))

	log.Printf("exec-agent listening on %s (work_root=%s, auth=%v)", listenAddr, workRoot, token != "")
	srv := &http.Server{Addr: listenAddr, Handler: mux}
	log.Fatal(srv.ListenAndServe())
}

// --- middleware ----------------------------------------------------------

func auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if got != token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }

// logicalRoot is the path the PentAGI framework thinks of as the container root.
// The exec-agent maps it onto the physical workRoot (which equals /work in a real
// deployment but can point elsewhere for local testing).
const logicalRoot = "/work"

// safePath resolves the framework path p onto the physical workRoot and
// guarantees the result stays inside workRoot.
func safePath(p string) (string, error) {
	rootAbs, _ := filepath.Abs(workRoot)
	if p == "" {
		return rootAbs, nil
	}
	clean := filepath.Clean(p)
	switch {
	case clean == logicalRoot:
		clean = rootAbs
	case strings.HasPrefix(clean, logicalRoot+string(os.PathSeparator)):
		clean = filepath.Join(rootAbs, strings.TrimPrefix(clean, logicalRoot))
	case !filepath.IsAbs(clean):
		clean = filepath.Join(rootAbs, clean)
	}
	abs, _ := filepath.Abs(clean)
	if abs != rootAbs && !strings.HasPrefix(abs, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %s escapes work root", p)
	}
	return abs, nil
}

// --- workdir -------------------------------------------------------------

func handleWorkdir(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			FlowID int64 `json:"flow_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		dir, err := safePath(fmt.Sprintf("flow-%d", body.FlowID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		flowID := r.URL.Query().Get("flow_id")
		if _, err := strconv.ParseInt(flowID, 10, 64); err != nil {
			http.Error(w, "invalid flow_id", http.StatusBadRequest)
			return
		}
		dir, err := safePath("flow-" + flowID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- exec ----------------------------------------------------------------

func handleExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cmd     []string `json:"cmd"`
		WorkDir string   `json:"workdir"`
		Tty     bool     `json:"tty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.Cmd) == 0 {
		http.Error(w, "empty cmd", http.StatusBadRequest)
		return
	}
	wd := workRoot
	if req.WorkDir != "" {
		var err error
		wd, err = safePath(req.WorkDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = os.MkdirAll(wd, 0o755)
	}

	// Stream raw combined stdout+stderr, matching the docker Tty attach the
	// caller expects (io.Copy of a single unframed stream).
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)

	cmd := exec.CommandContext(r.Context(), req.Cmd[0], req.Cmd[1:]...)
	cmd.Dir = wd
	fw := &flushWriter{w: w}
	if f, ok := w.(http.Flusher); ok {
		fw.f = f
	}
	cmd.Stdout = fw
	cmd.Stderr = fw

	if err := cmd.Run(); err != nil {
		// Surface non-zero exit / spawn errors inline (the caller reads the
		// combined stream and does not consult a separate exit channel).
		fmt.Fprintf(fw, "\n[exec-agent] command exited with error: %v\n", err)
	}
}

type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}

// --- stat / ls -----------------------------------------------------------

func statOf(p string) (container.PathStat, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return container.PathStat{}, err
	}
	link := ""
	if fi.Mode()&os.ModeSymlink != 0 {
		link, _ = os.Readlink(p)
	}
	return container.PathStat{
		Name:       fi.Name(),
		Size:       fi.Size(),
		Mode:       fi.Mode(),
		Mtime:      fi.ModTime(),
		LinkTarget: link,
	}, nil
}

func handleStat(w http.ResponseWriter, r *http.Request) {
	p, err := safePath(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	st, err := statOf(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, st)
}

func handleLs(w http.ResponseWriter, r *http.Request) {
	p, err := safePath(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	out := make([]container.PathStat, 0, len(entries))
	for _, e := range entries {
		if st, err := statOf(filepath.Join(p, e.Name())); err == nil {
			out = append(out, st)
		}
	}
	writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// --- filesystem tar transfer --------------------------------------------

func handleFS(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		dst, err := safePath(r.URL.Query().Get("path"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.MkdirAll(dst, 0o755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := extractTar(r.Body, dst); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		src, err := safePath(r.URL.Query().Get("path"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if st, serr := statOf(src); serr == nil {
			if b, merr := json.Marshal(st); merr == nil {
				w.Header().Set("X-Path-Stat", string(b))
			}
		}
		w.Header().Set("Content-Type", "application/x-tar")
		w.WriteHeader(http.StatusOK)
		if err := writeTar(w, src); err != nil {
			log.Printf("writeTar %s: %v", src, err)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func extractTar(r io.Reader, dst string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.Clean("/"+hdr.Name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
}

func writeTar(w io.Writer, src string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	base := filepath.Base(src)
	if !info.IsDir() {
		return addFileToTar(tw, src, base, info)
	}
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		name := filepath.Join(base, rel)
		if fi.IsDir() {
			hdr, _ := tar.FileInfoHeader(fi, "")
			hdr.Name = name + "/"
			return tw.WriteHeader(hdr)
		}
		return addFileToTar(tw, p, name, fi)
	})
}

func addFileToTar(tw *tar.Writer, p, name string, fi os.FileInfo) error {
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}
