package nanomesh

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

const (
	labelEnabled = "swarmex.mesh.enabled"
	labelNetwork = "swarmex.mesh.network" // EasyTier network name
	labelSecret  = "swarmex.mesh.secret"  // EasyTier network secret

	defaultNetwork = "swarmex"
)

// MeshConfig parsed from Docker service labels.
type MeshConfig struct {
	NetworkName   string
	NetworkSecret string
}

type peerState struct {
	config MeshConfig
	name   string
}

// Mesh manages EasyTier peers for Docker Swarm services.
type Mesh struct {
	docker       *client.Client
	logger       *slog.Logger
	peers        map[string]*peerState // keyed by service ID
	mu           sync.Mutex
	easytierBin  string // path to easytier-core binary
	peerEndpoint string // EasyTier peer endpoint to connect to
}

// New creates a Mesh manager.
func New(cli *client.Client, easytierBin, peerEndpoint string, logger *slog.Logger) *Mesh {
	return &Mesh{
		docker:       cli,
		logger:       logger,
		peers:        make(map[string]*peerState),
		easytierBin:  easytierBin,
		peerEndpoint: peerEndpoint,
	}
}

// HandleEvent processes Docker service events.
func (m *Mesh) HandleEvent(ctx context.Context, event events.Message) {
	if event.Type != events.ServiceEventType {
		return
	}
	switch event.Action {
	case events.ActionCreate, events.ActionUpdate:
		m.reconcile(ctx, event.Actor.ID)
	case events.ActionRemove:
		m.removePeer(event.Actor.ID)
	}
}

func (m *Mesh) reconcile(ctx context.Context, serviceID string) {
	svc, _, err := m.docker.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return
	}
	labels := svc.Spec.Labels
	if labels[labelEnabled] != "true" {
		m.removePeer(serviceID)
		return
	}

	cfg := parseMeshConfig(labels)

	m.mu.Lock()
	_, exists := m.peers[serviceID]
	m.peers[serviceID] = &peerState{config: cfg, name: svc.Spec.Name}
	m.mu.Unlock()

	if !exists {
		m.logger.Info("registering mesh peer", "service", svc.Spec.Name, "network", cfg.NetworkName)
		m.registerPeer(ctx, svc.Spec.Name, cfg)
	}
}

func (m *Mesh) registerPeer(ctx context.Context, serviceName string, cfg MeshConfig) {
	// Call easytier-core CLI to join the mesh network
	args := []string{
		"--network-name", cfg.NetworkName,
		"--network-secret", cfg.NetworkSecret,
		"--hostname", serviceName,
	}
	if m.peerEndpoint != "" {
		args = append(args, "-p", m.peerEndpoint)
	}

	cmd := exec.CommandContext(ctx, m.easytierBin, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error("easytier register failed",
			"service", serviceName, "error", err, "output", strings.TrimSpace(string(output)))
		return
	}
	m.logger.Info("mesh peer registered", "service", serviceName)
}

func (m *Mesh) removePeer(serviceID string) {
	m.mu.Lock()
	state, ok := m.peers[serviceID]
	if ok {
		delete(m.peers, serviceID)
	}
	m.mu.Unlock()

	if ok {
		m.logger.Info("mesh peer removed", "service", state.name)
	}
}

// ActivePeers returns count of active mesh peers.
func (m *Mesh) ActivePeers() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.peers)
}

// OverlayMonitor periodically checks overlay network health by verifying that
// services can be resolved via Docker's embedded DNS. If a service becomes
// unreachable, it force-updates the service to trigger task rescheduling.
func (m *Mesh) OverlayMonitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	failCounts := make(map[string]int) // serviceID → consecutive failures

	for {
		select {
		case <-ticker.C:
			m.checkOverlayHealth(ctx, failCounts)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Mesh) checkOverlayHealth(ctx context.Context, failCounts map[string]int) {
	services, err := m.docker.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		m.logger.Error("overlay monitor: failed to list services", "error", err)
		return
	}

	for _, svc := range services {
		if svc.Spec.Labels["swarmex.overlay.monitor"] == "false" {
			continue
		}

		// Only check services with running tasks
		desired := uint64(0)
		if svc.Spec.Mode.Replicated != nil && svc.Spec.Mode.Replicated.Replicas != nil {
			desired = *svc.Spec.Mode.Replicated.Replicas
		}
		if desired == 0 {
			continue
		}

		tasks, err := m.docker.TaskList(ctx, types.TaskListOptions{
			Filters: filters.NewArgs(
				filters.Arg("service", svc.ID),
				filters.Arg("desired-state", "running"),
			),
		})
		if err != nil {
			continue
		}

		running := 0
		for _, t := range tasks {
			if t.Status.State == swarm.TaskStateRunning {
				running++
			}
		}

		if running == 0 && desired > 0 {
			failCounts[svc.ID]++
			if failCounts[svc.ID] >= 3 {
				m.logger.Warn("overlay monitor: service has no running tasks, force-updating",
					"service", svc.Spec.Name, "desired", desired, "consecutive_checks", failCounts[svc.ID])
				svc.Spec.TaskTemplate.ForceUpdate++
				_, err := m.docker.ServiceUpdate(ctx, svc.ID, svc.Version, svc.Spec, types.ServiceUpdateOptions{})
				if err != nil {
					m.logger.Error("overlay monitor: force-update failed", "service", svc.Spec.Name, "error", err)
				}
				failCounts[svc.ID] = 0
			}
		} else {
			delete(failCounts, svc.ID)
		}
	}

	// Prune stale network state periodically
	m.pruneStaleNetworks(ctx)
}

func (m *Mesh) pruneStaleNetworks(ctx context.Context) {
	networks, err := m.docker.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("driver", "overlay")),
	})
	if err != nil {
		return
	}

	for _, net := range networks {
		if net.Name == "ingress" {
			continue
		}
		// Check for networks with no connected containers (stale)
		info, err := m.docker.NetworkInspect(ctx, net.ID, types.NetworkInspectOptions{})
		if err != nil {
			continue
		}
		if len(info.Containers) == 0 && len(info.Services) == 0 && !info.Internal {
			m.logger.Info("overlay monitor: pruning stale network", "network", net.Name)
			_ = m.docker.NetworkRemove(ctx, net.ID)
		}
	}
}

func parseMeshConfig(labels map[string]string) MeshConfig {
	cfg := MeshConfig{
		NetworkName:   defaultNetwork,
		NetworkSecret: "",
	}
	if v, ok := labels[labelNetwork]; ok {
		cfg.NetworkName = v
	}
	if v, ok := labels[labelSecret]; ok {
		cfg.NetworkSecret = v
	}
	return cfg
}

// keep fmt import used
var _ = fmt.Sprintf
