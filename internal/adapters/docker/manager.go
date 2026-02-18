package docker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/manthysbr/auleOS/internal/core/ports"
)

const (
	containerSockDir = "/var/run/aule"
	watchdogSockName = "watchdog.sock"
	containerUser    = "aule"
)

type Manager struct {
	cli              *client.Client
	baseSocketDir    string
	baseWorkspaceDir string
	hostUser         string
}

// NewManager creates a new Docker manager
func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}

	uid := os.Getuid()
	gid := os.Getgid()

	return &Manager{
		cli:              cli,
		baseSocketDir:    filepath.Join(home, ".aule", "sockets"),
		baseWorkspaceDir: filepath.Join(home, ".aule", "workspaces"),
		hostUser:         fmt.Sprintf("%d:%d", uid, gid),
	}, nil
}

// Ensure Manager implements WorkerManager
var _ ports.WorkerManager = (*Manager)(nil)

func (m *Manager) Spawn(ctx context.Context, spec domain.WorkerSpec) (domain.WorkerID, error) {
	id := domain.WorkerID(uuid.New().String())

	// 1. Prepare Host Directories
	socketDir := filepath.Join(m.baseSocketDir, string(id))
	workspaceDir := filepath.Join(m.baseWorkspaceDir, string(id))

	if err := os.MkdirAll(socketDir, 0777); err != nil {
		return "", fmt.Errorf("failed to create socket dir: %w", err)
	}
	// Ensure chmod for 0777 because MkdirAll might be restricted by umask
	_ = os.Chmod(socketDir, 0777)

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		_ = os.RemoveAll(socketDir)
		return "", fmt.Errorf("failed to create workspace dir: %w", err)
	}

	// 2. Prepare Configs

	// Convert Env map to slice
	envSlice := []string{
		fmt.Sprintf("WATCHDOG_SOCKET_PATH=%s/%s", containerSockDir, watchdogSockName),
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	for k, v := range spec.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}
	if spec.AgentPrompt != "" {
		envSlice = append(envSlice, fmt.Sprintf("AULE_AGENT_PROMPT=%s", spec.AgentPrompt))
	}

	cfg := &container.Config{
		Image:        spec.Image,
		Cmd:          spec.Command,
		Env:          envSlice,
		User:         m.hostUser,
		Tty:          false,
		OpenStdin:    false,
		AttachStdout: false,
		AttachStderr: false,
		Labels: map[string]string{
			"aule.managed": "true",
			"aule.job_id":  string(id),
		},
	}

	// Prepare Binds (Volumes)
	// 1. Workspace Mount (Always)
	binds := []string{
		fmt.Sprintf("%s:/workspace", workspaceDir), // Fixed variable name from wsPath to workspaceDir
		fmt.Sprintf("%s:%s", socketDir, containerSockDir),
	}

	// 2. Custom Bind Mounts from Spec
	for hostPath, containerPath := range spec.BindMounts {
		binds = append(binds, fmt.Sprintf("%s:%s:ro", hostPath, containerPath)) // Default to ReadOnly for safety
	}

	hostCfg := &container.HostConfig{
		NetworkMode: "none", // STRICT SECURITY RULE
		Binds:       binds,
		Resources:   container.Resources{
			// TODO: Add CPU/Mem limit logic based on Spec
			// NanoCPUs: int64(spec.ResourceCPU * 1e9),
			// Memory:   spec.ResourceMem,
		},
		ReadonlyRootfs: spec.ReadonlyRootfs, // default false — set true only for hardened workers
		Tmpfs: map[string]string{
			"/tmp": "rw,noexec,nosuid,size=64m", // Allow writable /tmp
		},
	}

	netCfg := &network.NetworkingConfig{} // None

	// 3. Create Container
	// We might need to pull image first if not present, but for now assuming it exists or implicit pull
	// (Client.ContainerCreate doesn't auto-pull, usually. But let's assume images are pre-pulled for M1)

	resp, err := m.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, "aule-worker-"+string(id))
	if client.IsErrNotFound(err) {
		// Try to pull
		reader, pullErr := m.cli.ImagePull(ctx, spec.Image, image.PullOptions{})
		if pullErr != nil {
			m.cleanup(socketDir, workspaceDir)
			return "", fmt.Errorf("failed to pull image %s: %w", spec.Image, pullErr)
		}
		io.Copy(io.Discard, reader)
		reader.Close()
		// Retry create
		resp, err = m.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, "aule-worker-"+string(id))
	}

	if err != nil {
		m.cleanup(socketDir, workspaceDir)
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// 4. Start
	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = m.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		m.cleanup(socketDir, workspaceDir)
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	// Success
	// Note: We return the internal ID (UUID), not the Docker Container ID.
	// We might want to store the mapping in the Repo, or use the Label to find it back.
	// For simplicity, we used "aule-worker-<UUID>" as name, so we can reconstruct it.

	return id, nil
}

func (m *Manager) cleanup(paths ...string) {
	for _, p := range paths {
		_ = os.RemoveAll(p)
	}
}

func (m *Manager) HealthCheck(ctx context.Context, id domain.WorkerID) (domain.HealthStatus, error) {
	// 1. Check if container is running via Docker API
	cID := "aule-worker-" + string(id)
	inspect, err := m.cli.ContainerInspect(ctx, cID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return domain.HealthStatusExited, nil // Or Unknown
		}
		return domain.HealthStatusUnknown, err
	}

	if !inspect.State.Running {
		return domain.HealthStatusExited, nil
	}

	// 2. Ping Watchdog via Unix Socket
	socketPath := filepath.Join(m.baseSocketDir, string(id), watchdogSockName)

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 500 * time.Millisecond,
	}

	resp, err := httpClient.Get("http://localhost/health")
	if err != nil {
		// Container running but watchdog cannot be reached?
		// Maybe it's still starting up.
		return domain.HealthStatusStarting, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return domain.HealthStatusHealthy, nil
	}

	return domain.HealthStatusUnhealthy, nil
}

func (m *Manager) Kill(ctx context.Context, id domain.WorkerID) error {
	cID := "aule-worker-" + string(id)

	// Force remove container (Stop + Remove)
	err := m.cli.ContainerRemove(ctx, cID, container.RemoveOptions{Force: true})
	if err != nil && !client.IsErrNotFound(err) {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Cleanup bind mounts
	m.cleanup(
		filepath.Join(m.baseSocketDir, string(id)),
		filepath.Join(m.baseWorkspaceDir, string(id)),
	)

	return nil
}

func (m *Manager) List(ctx context.Context) ([]domain.Worker, error) {
	// List containers with label aule.managed=true
	containers, err := m.cli.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: makeFilters(map[string]string{
			"label": "aule.managed=true",
		}),
	})
	if err != nil {
		return nil, err
	}

	var workers []domain.Worker
	for _, c := range containers {
		// Parse ID from label or name
		idStr := c.Labels["aule.job_id"]
		if idStr == "" {
			continue
		}

		status := domain.HealthStatusUnknown
		switch c.State {
		case "running":
			status = domain.HealthStatusHealthy // Optimistic
		case "exited", "dead":
			status = domain.HealthStatusExited
		}

		workers = append(workers, domain.Worker{
			ID:     domain.WorkerID(idStr),
			Status: status,
			Metadata: map[string]string{
				"docker_id": c.ID,
				"image":     c.Image,
			},
		})
	}

	return workers, nil
}

func (m *Manager) GetLogs(ctx context.Context, id domain.WorkerID) (io.ReadCloser, error) {
	cID := "aule-worker-" + string(id)
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	}

	return m.cli.ContainerLogs(ctx, cID, opts)
}

// Helper to construct list filters
func makeFilters(m map[string]string) filters.Args {
	args := filters.NewArgs()
	for k, v := range m {
		args.Add(k, v)
	}
	return args
}

// GetWorkerIP returns the IP address of a worker container
func (m *Manager) GetWorkerIP(ctx context.Context, workerID domain.WorkerID) (string, error) {
	inspect, err := m.cli.ContainerInspect(ctx, string(workerID))
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	// Get IP from first network (usually bridge)
	for _, network := range inspect.NetworkSettings.Networks {
		if network.IPAddress != "" {
			return network.IPAddress, nil
		}
	}

	return "", fmt.Errorf("no IP address found for worker %s", workerID)
}
