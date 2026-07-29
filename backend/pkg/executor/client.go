// Package executor provides an alternative implementation of docker.DockerClient
// that does NOT spawn containers via the Docker API. Instead it talks to a
// long-running "exec-agent" (a small HTTP service that ships inside a resident
// Kali/tools container or pod) and runs every agent command as an exec against
// that single environment.
//
// This is what makes PentAGI deployable on platforms that forbid Docker-socket
// / host-runtime access (e.g. restricted Kubernetes pods on AWS EKS): the
// orchestrator never touches the host Docker daemon; it only makes HTTP calls to
// the executor over the cluster network.
//
// Selection is controlled by EXECUTION_BACKEND=executor (default is "docker",
// which keeps the original local behaviour untouched).
//
// Isolation model: all flows share one executor. Per-flow separation is done by
// giving each flow its own working directory /work/flow-<id> and remapping the
// framework's /work paths into it. This trades the docker backend's per-flow
// container isolation for simplicity — a documented, intentional tradeoff.
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"pentagi/pkg/config"
	"pentagi/pkg/database"
	"pentagi/pkg/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
)

// Compile-time proof that ExecAgentClient satisfies the same interface the rest
// of PentAGI depends on, so it can be dropped in wherever docker.DockerClient is.
var _ docker.DockerClient = (*ExecAgentClient)(nil)

// WorkFolderPathInContainer mirrors docker.WorkFolderPathInContainer. The
// framework treats this as the container root; we remap it per flow.
const workFolderRoot = "/work"

const defaultRequestTimeout = 30 * time.Second

type execEntry struct {
	containerName string
	cmd           []string
	workDir       string // already remapped to the flow workdir
}

// ExecAgentClient implements docker.DockerClient by delegating to a remote
// exec-agent over HTTP.
type ExecAgentClient struct {
	db       database.Querier
	logger   *logrus.Logger
	baseURL  string
	token    string
	defImage string
	http     *http.Client

	mu       sync.Mutex
	flowByID map[string]int64     // containerName -> flowID (for path remapping)
	execs    map[string]execEntry // execID -> pending exec
	execSeq  int64
}

// NewExecAgentClient builds an executor-backed DockerClient-compatible client.
func NewExecAgentClient(ctx context.Context, db database.Querier, cfg *config.Config) (*ExecAgentClient, error) {
	if cfg.ExecutorURL == "" {
		return nil, fmt.Errorf("EXECUTOR_URL is required when EXECUTION_BACKEND=executor")
	}
	base := strings.TrimRight(cfg.ExecutorURL, "/")
	c := &ExecAgentClient{
		db:       db,
		logger:   logrus.StandardLogger(),
		baseURL:  base,
		token:    cfg.ExecutorToken,
		defImage: cfg.ExecutorDefaultImage,
		http:     &http.Client{Timeout: 0}, // per-request contexts control timeouts
		flowByID: make(map[string]int64),
		execs:    make(map[string]execEntry),
	}
	// Probe the executor at startup, but tolerate it not being ready yet.
	//
	// On an orchestrated platform (e.g. Kubernetes) pentagi and the exec-agent
	// come up as separate pods with no guaranteed ordering, so a hard failure
	// here would crash-loop the whole app whenever the executor lags a few
	// seconds behind — which in turn fails the deploy's readiness/smoke check.
	// We retry for a bounded window and, if the executor is still unreachable,
	// start in a degraded mode: the HTTP server (and /healthz) come up so the
	// app is deployable, while executor-backed actions surface their own error
	// until the exec-agent becomes reachable. Misconfiguration is still loud in
	// the logs rather than silent.
	if err := c.pingWithRetry(ctx, execAgentStartupProbeTimeout, execAgentStartupProbeInterval); err != nil {
		c.logger.WithField("executor_url", base).
			WithError(err).
			Warn("execution backend: exec-agent not reachable at startup; continuing in degraded mode (health/UI up, executor actions will retry)")
		return c, nil
	}
	c.logger.WithField("executor_url", base).Info("execution backend: remote exec-agent")
	return c, nil
}

const (
	// How long to wait for the exec-agent to become reachable at startup before
	// giving up and booting in degraded mode, and how often to re-probe.
	execAgentStartupProbeTimeout  = 90 * time.Second
	execAgentStartupProbeInterval = 3 * time.Second
)

// pingWithRetry probes the exec-agent until it responds healthy, the timeout
// elapses, or ctx is cancelled. It returns nil on the first successful probe.
func (c *ExecAgentClient) pingWithRetry(ctx context.Context, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for attempt := 1; ; attempt++ {
		if err := c.ping(ctx); err == nil {
			if attempt > 1 {
				c.logger.WithField("executor_url", c.baseURL).
					WithField("attempts", attempt).
					Info("execution backend: exec-agent became reachable")
			}
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("exec-agent unreachable after %s: %w", timeout, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// --- helpers -------------------------------------------------------------

func (c *ExecAgentClient) ping(ctx context.Context) error {
	pctx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()
	resp, err := c.do(pctx, http.MethodGet, "/v1/health", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected health status %d", resp.StatusCode)
	}
	return nil
}

func (c *ExecAgentClient) do(ctx context.Context, method, p string, query url.Values, body io.Reader) (*http.Response, error) {
	u := c.baseURL + p
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

func (c *ExecAgentClient) doJSON(ctx context.Context, method, p string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	rctx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()
	resp, err := c.do(rctx, method, p, nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("exec-agent %s %s -> %d: %s", method, p, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *ExecAgentClient) flowDir(flowID int64) string {
	return path.Join(workFolderRoot, fmt.Sprintf("flow-%d", flowID))
}

// remap rewrites a framework path rooted at /work into the flow's private dir.
func (c *ExecAgentClient) remap(containerName, p string) string {
	c.mu.Lock()
	flowID, ok := c.flowByID[containerName]
	c.mu.Unlock()
	if !ok {
		return p
	}
	fd := c.flowDir(flowID)
	if p == "" || p == workFolderRoot {
		return fd
	}
	if strings.HasPrefix(p, workFolderRoot+"/") {
		return path.Join(fd, strings.TrimPrefix(p, workFolderRoot))
	}
	return p
}

// --- container lifecycle -------------------------------------------------

func (c *ExecAgentClient) RunContainer(
	ctx context.Context,
	containerName string,
	containerType database.ContainerType,
	flowID int64,
	config *container.Config,
	hostConfig *container.HostConfig,
) (database.Container, error) {
	if config == nil {
		return database.Container{}, fmt.Errorf("no config found for container %s", containerName)
	}

	logger := c.logger.WithContext(ctx).WithFields(logrus.Fields{
		"name":    containerName,
		"type":    containerType,
		"flow_id": flowID,
		"backend": "executor",
	})
	logger.Info("running container (executor-backed, no container spawned)")

	// Ensure the flow's private working directory exists on the executor.
	if err := c.doJSON(ctx, http.MethodPost, "/v1/workdir", map[string]any{"flow_id": flowID}, nil); err != nil {
		return database.Container{}, fmt.Errorf("failed to create flow workdir on executor: %w", err)
	}

	dbContainer, err := c.db.CreateContainer(ctx, database.CreateContainerParams{
		Type:     containerType,
		Name:     containerName,
		Image:    c.defImage,
		Status:   database.ContainerStatusStarting,
		FlowID:   flowID,
		LocalID:  database.StringToNullString(fmt.Sprintf("executor-flow-%d", flowID)),
		LocalDir: database.StringToNullString(c.flowDir(flowID)),
	})
	if err != nil {
		return database.Container{}, fmt.Errorf("failed to create container in database: %w", err)
	}

	dbContainer, err = c.db.UpdateContainerStatusLocalID(ctx, database.UpdateContainerStatusLocalIDParams{
		Status:  database.ContainerStatusRunning,
		LocalID: database.StringToNullString(fmt.Sprintf("executor-flow-%d", flowID)),
		ID:      dbContainer.ID,
	})
	if err != nil {
		return database.Container{}, fmt.Errorf("failed to update container status: %w", err)
	}

	c.mu.Lock()
	c.flowByID[containerName] = flowID
	c.mu.Unlock()

	logger.Info("container ready (executor-backed)")
	return dbContainer, nil
}

func (c *ExecAgentClient) StopContainer(ctx context.Context, containerID string, dbID int64) error {
	// Nothing to stop: the executor is shared and long-running. Just record it.
	if _, err := c.db.UpdateContainerStatus(ctx, database.UpdateContainerStatusParams{
		Status: database.ContainerStatusStopped,
		ID:     dbID,
	}); err != nil {
		return fmt.Errorf("failed to update container status: %w", err)
	}
	return nil
}

func (c *ExecAgentClient) RemoveContainer(ctx context.Context, containerID string, dbID int64) error {
	// containerID is the LocalID we set in RunContainer: "executor-flow-<flowID>".
	// Drop the flow's working directory on the executor (best-effort).
	if flowStr := strings.TrimPrefix(containerID, "executor-flow-"); flowStr != containerID && flowStr != "" {
		q := url.Values{}
		q.Set("flow_id", flowStr)
		rctx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
		if resp, derr := c.do(rctx, http.MethodDelete, "/v1/workdir", q, nil); derr == nil {
			resp.Body.Close()
		}
		cancel()
	}

	c.mu.Lock()
	for name := range c.flowByID {
		if fmt.Sprintf("executor-flow-%d", c.flowByID[name]) == containerID {
			delete(c.flowByID, name)
		}
	}
	c.mu.Unlock()

	// Mirror the docker backend: mark the DB row deleted rather than dropping it.
	if _, err := c.db.UpdateContainerStatus(ctx, database.UpdateContainerStatusParams{
		Status: database.ContainerStatusDeleted,
		ID:     dbID,
	}); err != nil {
		return fmt.Errorf("failed to update container status to deleted: %w", err)
	}
	return nil
}

func (c *ExecAgentClient) Cleanup(ctx context.Context) error {
	// The executor persists across restarts; there is nothing host-side to reap.
	return nil
}

func (c *ExecAgentClient) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	if err := c.ping(ctx); err != nil {
		return false, nil
	}
	return true, nil
}

func (c *ExecAgentClient) GetDefaultImage() string {
	return c.defImage
}

// --- command execution ---------------------------------------------------

func (c *ExecAgentClient) ContainerExecCreate(
	ctx context.Context,
	containerName string,
	cfg container.ExecOptions,
) (container.ExecCreateResponse, error) {
	workDir := c.remap(containerName, cfg.WorkingDir)
	if workDir == "" {
		c.mu.Lock()
		if fid, ok := c.flowByID[containerName]; ok {
			workDir = c.flowDir(fid)
		}
		c.mu.Unlock()
	}

	c.mu.Lock()
	c.execSeq++
	execID := fmt.Sprintf("exec-%d", c.execSeq)
	c.execs[execID] = execEntry{containerName: containerName, cmd: cfg.Cmd, workDir: workDir}
	c.mu.Unlock()

	return container.ExecCreateResponse{ID: execID}, nil
}

func (c *ExecAgentClient) ContainerExecAttach(
	ctx context.Context,
	execID string,
	cfg container.ExecAttachOptions,
) (types.HijackedResponse, error) {
	c.mu.Lock()
	entry, ok := c.execs[execID]
	c.mu.Unlock()
	if !ok {
		return types.HijackedResponse{}, fmt.Errorf("unknown exec id %s", execID)
	}

	payload := map[string]any{
		"cmd":     entry.cmd,
		"workdir": entry.workDir,
		"tty":     true,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return types.HijackedResponse{}, err
	}

	// No timeout on the client here: the caller (terminal) applies its own
	// context deadline and calls Close() to unblock the stream.
	resp, err := c.do(ctx, http.MethodPost, "/v1/exec", nil, bytes.NewReader(b))
	if err != nil {
		return types.HijackedResponse{}, fmt.Errorf("failed to start exec on executor: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return types.HijackedResponse{}, fmt.Errorf("executor exec failed %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	// Wrap the streaming body as a net.Conn so NewHijackedResponse can read it
	// and Close() tears it down.
	conn := &bodyConn{body: resp.Body}
	return types.NewHijackedResponse(conn, "application/vnd.docker.raw-stream"), nil
}

func (c *ExecAgentClient) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	c.mu.Lock()
	delete(c.execs, execID)
	c.mu.Unlock()
	// The terminal only checks for a non-nil error; exit code is not consumed.
	return container.ExecInspect{ExecID: execID, Running: false}, nil
}

// --- filesystem ----------------------------------------------------------

func (c *ExecAgentClient) ContainerStatPath(ctx context.Context, containerID string, p string) (container.PathStat, error) {
	q := url.Values{}
	q.Set("path", c.remap(containerID, p))
	rctx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()
	resp, err := c.do(rctx, http.MethodGet, "/v1/stat", q, nil)
	if err != nil {
		return container.PathStat{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return container.PathStat{}, fmt.Errorf("stat %s -> %d", p, resp.StatusCode)
	}
	var st container.PathStat
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return container.PathStat{}, err
	}
	return st, nil
}

func (c *ExecAgentClient) ListContainerDir(ctx context.Context, containerID string, dirPath string) ([]container.PathStat, error) {
	q := url.Values{}
	q.Set("path", c.remap(containerID, dirPath))
	rctx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()
	resp, err := c.do(rctx, http.MethodGet, "/v1/ls", q, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ls %s -> %d", dirPath, resp.StatusCode)
	}
	var out []container.PathStat
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ExecAgentClient) CopyToContainer(
	ctx context.Context,
	containerID string,
	dstPath string,
	content io.Reader,
	options container.CopyToContainerOptions,
) error {
	q := url.Values{}
	q.Set("path", c.remap(containerID, dstPath))
	rctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	resp, err := c.do(rctx, http.MethodPut, "/v1/fs", q, content)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("copy-to %s -> %d: %s", dstPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (c *ExecAgentClient) CopyFromContainer(
	ctx context.Context,
	containerID string,
	srcPath string,
) (io.ReadCloser, container.PathStat, error) {
	q := url.Values{}
	q.Set("path", c.remap(containerID, srcPath))
	// No timeout cancel here: the caller consumes and closes the stream.
	resp, err := c.do(ctx, http.MethodGet, "/v1/fs", q, nil)
	if err != nil {
		return nil, container.PathStat{}, err
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		return nil, container.PathStat{}, fmt.Errorf("copy-from %s -> %d: %s", srcPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	var st container.PathStat
	if hdr := resp.Header.Get("X-Path-Stat"); hdr != "" {
		_ = json.Unmarshal([]byte(hdr), &st)
	}
	return resp.Body, st, nil
}

// --- bodyConn: minimal net.Conn over an http response body --------------

type bodyConn struct {
	body io.ReadCloser
}

func (b *bodyConn) Read(p []byte) (int, error)  { return b.body.Read(p) }
func (b *bodyConn) Write(p []byte) (int, error) { return 0, io.ErrClosedPipe }
func (b *bodyConn) Close() error                { return b.body.Close() }
func (b *bodyConn) LocalAddr() net.Addr         { return dummyAddr{} }
func (b *bodyConn) RemoteAddr() net.Addr        { return dummyAddr{} }
func (b *bodyConn) SetDeadline(time.Time) error      { return nil }
func (b *bodyConn) SetReadDeadline(time.Time) error  { return nil }
func (b *bodyConn) SetWriteDeadline(time.Time) error { return nil }

type dummyAddr struct{}

func (dummyAddr) Network() string { return "executor" }
func (dummyAddr) String() string  { return "exec-agent" }
