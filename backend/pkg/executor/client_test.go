package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
)

// newTestClient wires an ExecAgentClient to a fake exec-agent, bypassing the
// DB-dependent constructor so we can exercise the exec/file code paths directly.
func newTestClient(baseURL string) *ExecAgentClient {
	return &ExecAgentClient{
		logger:   logrus.StandardLogger(),
		baseURL:  strings.TrimRight(baseURL, "/"),
		token:    "tok",
		defImage: "vxcontrol/kali-linux:latest",
		http:     &http.Client{},
		flowByID: map[string]int64{"pentagi-terminal-7": 7},
		execs:    map[string]execEntry{},
	}
}

func TestExecClientExecStreamsAndRemapsPaths(t *testing.T) {
	var gotAuth, gotExecWorkdir, gotStatPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/v1/health":
			w.WriteHeader(http.StatusOK)
		case "/v1/exec":
			var body struct {
				Cmd     []string `json:"cmd"`
				WorkDir string   `json:"workdir"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotExecWorkdir = body.WorkDir
			// stream raw combined output, like a Tty attach
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "PORT   STATE SERVICE\n80/tcp open  http\n")
		case "/v1/stat":
			gotStatPath = r.URL.Query().Get("path")
			json.NewEncoder(w).Encode(container.PathStat{Name: "report.txt", Size: 42})
		case "/v1/ls":
			json.NewEncoder(w).Encode([]container.PathStat{{Name: "a"}, {Name: "b"}})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	ctx := context.Background()

	// 1) exec: Create -> Attach -> read the streamed output -> Inspect
	created, err := c.ContainerExecCreate(ctx, "pentagi-terminal-7", container.ExecOptions{
		Cmd:        []string{"sh", "-c", "nmap -F scanme.nmap.org"},
		WorkingDir: "/work", // must be remapped to /work/flow-7
		Tty:        true,
	})
	if err != nil {
		t.Fatalf("ContainerExecCreate: %v", err)
	}
	hj, err := c.ContainerExecAttach(ctx, created.ID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		t.Fatalf("ContainerExecAttach: %v", err)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, hj.Reader); err != nil {
		t.Fatalf("read exec stream: %v", err)
	}
	hj.Close()
	if !strings.Contains(out.String(), "80/tcp open") {
		t.Fatalf("unexpected exec output: %q", out.String())
	}
	if _, err := c.ContainerExecInspect(ctx, created.ID); err != nil {
		t.Fatalf("ContainerExecInspect: %v", err)
	}

	// path remap: /work must have been rewritten to the flow's private dir
	if gotExecWorkdir != "/work/flow-7" {
		t.Fatalf("workdir not remapped: got %q want /work/flow-7", gotExecWorkdir)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth header not sent: got %q", gotAuth)
	}

	// 2) stat: path under /work is remapped
	st, err := c.ContainerStatPath(ctx, "pentagi-terminal-7", "/work/report.txt")
	if err != nil {
		t.Fatalf("ContainerStatPath: %v", err)
	}
	if st.Name != "report.txt" || st.Size != 42 {
		t.Fatalf("unexpected stat: %+v", st)
	}
	if gotStatPath != "/work/flow-7/report.txt" {
		t.Fatalf("stat path not remapped: got %q", gotStatPath)
	}

	// 3) ls
	entries, err := c.ListContainerDir(ctx, "pentagi-terminal-7", "/work")
	if err != nil {
		t.Fatalf("ListContainerDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// 4) default image passthrough
	if c.GetDefaultImage() != "vxcontrol/kali-linux:latest" {
		t.Fatalf("GetDefaultImage wrong: %s", c.GetDefaultImage())
	}
}

func TestRemapLeavesNonWorkPathsAlone(t *testing.T) {
	c := newTestClient("http://x")
	if got := c.remap("pentagi-terminal-7", "/work/uploads"); got != "/work/flow-7/uploads" {
		t.Fatalf("remap /work/uploads = %q", got)
	}
	if got := c.remap("pentagi-terminal-7", "/etc/passwd"); got != "/etc/passwd" {
		t.Fatalf("remap should not touch /etc/passwd, got %q", got)
	}
	if got := c.remap("unknown-container", "/work/x"); got != "/work/x" {
		t.Fatalf("unknown container should pass through, got %q", got)
	}
}
