# Swarmex Nano Mesh

Service mesh peer registration via EasyTier (WireGuard) for Docker Swarm.

Part of [Swarmex](https://github.com/ccvass/swarmex) — enterprise-grade orchestration for Docker Swarm.

## What It Does

Automatically registers mesh-enabled services as EasyTier peers, creating a WireGuard-based service mesh. Services in the same mesh network get encrypted peer-to-peer connectivity without manual configuration.

## Labels

```yaml
deploy:
  labels:
    swarmex.mesh.enabled: "true"         # Enable mesh registration
    swarmex.mesh.network: "production"   # Mesh network name
```

## How It Works

1. Watches for services with mesh labels via Docker events.
2. Resolves the service's container IPs and endpoints.
3. Registers each instance as an EasyTier peer in the configured network.
4. Deregisters peers when services are removed or scaled down.

## Quick Start

```bash
docker service update \
  --label-add swarmex.mesh.enabled=true \
  --label-add swarmex.mesh.network=production \
  my-app
```

## Verified

Peers registered successfully for all mesh-enabled services in the target network.

## License

Apache-2.0
