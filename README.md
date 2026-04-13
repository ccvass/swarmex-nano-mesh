# swarmex-nano-mesh

Lightweight WireGuard service mesh for Docker Swarm via EasyTier.

## What it does

Listens to Docker events, auto-registers/deregisters EasyTier mesh peers when services with mesh labels start or stop. Provides encrypted service-to-service communication without manual WireGuard configuration.

## Why it matters

Kubernetes has Istio and Linkerd for service mesh with mTLS. Docker Swarm has encrypted overlay networks but no service mesh with automatic peer management. This controller wraps EasyTier (10.8K stars, WireGuard-based) to provide automatic mesh networking.

## Verified

- ✅ Detected service with mesh labels
- ✅ Attempted peer registration with EasyTier
- ✅ EasyTier binary included in Docker image

## Configuration

```yaml
deploy:
  labels:
    swarmex.mesh.enabled: "true"
    swarmex.mesh.network: "my-mesh"
    swarmex.mesh.secret: "mesh-secret"
```
