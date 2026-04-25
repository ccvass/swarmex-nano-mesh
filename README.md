<p align="center"><img src="https://raw.githubusercontent.com/ccvass/swarmex/main/docs/assets/logo.svg" alt="Swarmex" width="400"></p>

[![Test, Build & Deploy](https://github.com/ccvass/swarmex-nano-mesh/actions/workflows/publish.yml/badge.svg)](https://github.com/ccvass/swarmex-nano-mesh/actions/workflows/publish.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

# Swarmex Nano Mesh

Service mesh peer registration via EasyTier (WireGuard) for Docker Swarm.

Part of [Swarmex](https://github.com/ccvass/swarmex) — enterprise-grade orchestration for Docker Swarm.

## What It Does

Automatically registers mesh-enabled services as EasyTier peers, creating a WireGuard-based service mesh. Also monitors overlay network health and auto-heals connectivity issues.

**v1.1.0**: Added overlay health monitor that detects services with 0 running tasks and force-updates them to trigger rescheduling. Prunes stale overlay networks.

## Labels

```yaml
deploy:
  labels:
    swarmex.mesh.enabled: "true"              # Enable mesh registration
    swarmex.mesh.network: "production"        # Mesh network name
    swarmex.overlay.monitor: "false"          # Disable overlay monitoring for this service
```

## How It Works

### Mesh Registration

1. Watches for services with mesh labels via Docker events.
2. Resolves the service's container IPs and endpoints.
3. Registers each instance as an EasyTier peer in the configured network.
4. Deregisters peers when services are removed or scaled down.

### Overlay Health Monitor

1. Every 30 seconds, checks all services for running task count.
2. If a service has 0 running tasks for 3 consecutive checks, force-updates it.
3. Periodically prunes stale overlay networks with no connected containers.
4. Disable per-service with label `swarmex.overlay.monitor=false`.

## Quick Start

```bash
docker service update \
  --label-add swarmex.mesh.enabled=true \
  --label-add swarmex.mesh.network=production \
  my-app
```

## Verified

- Peers registered successfully for all mesh-enabled services
- Overlay monitor detects and force-updates stuck services
- Stale overlay networks pruned automatically

## License

Apache-2.0
