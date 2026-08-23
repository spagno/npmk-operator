# NPMK Operator

[![CI](https://github.com/spagno/npmk-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/spagno/npmk-operator/actions/workflows/ci.yaml)
[![Release](https://github.com/spagno/npmk-operator/actions/workflows/release.yaml/badge.svg)](https://github.com/spagno/npmk-operator/actions/workflows/release.yaml)
[![codecov](https://codecov.io/github/spagno/npmk-operator/graph/badge.svg?token=S8MF8HY6A9)](https://codecov.io/github/spagno/npmk-operator)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.19+-326CE5?style=flat&logo=kubernetes&logoColor=white)](https://kubernetes.io/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

A Kubernetes operator that automatically manages [Nginx Proxy Manager](https://nginxproxymanager.com/) proxy hosts based on Kubernetes services, with automatic **Let's Encrypt SSL certificates** via DNS challenge.

## Overview

The NPMK Operator watches for `Npmko` custom resources and automatically:

- Discovers service endpoints in your cluster
- Creates/updates proxy hosts in Nginx Proxy Manager
- Provisions Let's Encrypt SSL certificates via DNS challenge
- Keeps everything in sync with periodic reconciliation

## Features

- **Automatic Proxy Host Management**: Creates and updates NPM proxy hosts based on Kubernetes resources
- **Dynamic Endpoint Discovery**: Automatically detects and uses service endpoints
- **Let's Encrypt SSL**: Automatic certificate provisioning via DNS challenge (Cloudflare, Route53, DigitalOcean, PowerDNS, etc.)
- **Multi-Domain Support**: Configure multiple domain names for a single service
- **Flexible Configuration**: Global and namespace-specific configuration via ConfigMaps and Secrets
- **Advanced Options**: Support for HTTP/2, HSTS, websockets, caching, and custom nginx configs

## Architecture

The operator consists of:

- A custom resource definition (CRD) for `Npmko` resources
- A controller that reconciles Npmko resources with NPM proxy hosts
- ConfigMap/Secret-based configuration system with namespace override capability
- NPM API client for proxy host and certificate management

## Custom Resource Definition

```yaml
apiVersion: npmko.io/v1alpha1
kind: Npmko
metadata:
  name: my-app
  namespace: production
spec:
  serviceName: my-app-service         # Required: Service to proxy
  domainNames:                         # Required: Domain names
    - myapp.example.com
    - www.myapp.example.com
  forwardScheme: http                  # Optional: http or https (default: http)
  ssl:                                 # Optional: Let's Encrypt SSL config
    enabled: true
    forceSSL: true
    dnsChallenge:
      provider: cloudflare             # DNS provider name
      credentialsSecretRef:
        name: cloudflare-credentials   # Secret with DNS credentials
        key: credentials               # Key in the Secret
      propagationSeconds: 30           # DNS propagation wait time
  cachingEnabled: false                # Optional: Enable caching
  blockExploits: true                  # Optional: Block common exploits (default: true)
  allowWebsockets: true                # Optional: Allow websocket upgrade (default: true)
  http2Support: true                   # Optional: Enable HTTP/2 (default: true)
  hstsEnabled: false                   # Optional: Enable HSTS
  hstsSubdomains: false                # Optional: Include subdomains in HSTS
  advancedConfig: |                    # Optional: Custom nginx config
    proxy_set_header X-Custom-Header "value";
```

## Configuration

### Global Configuration

Create a ConfigMap and Secret in the operator's namespace (`npmk-operator-system`):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: npmko-config
  namespace: npmk-operator-system
data:
  npm-url: "http://nginx-proxy-manager:81"
  npm-email: "admin@example.com"
---
apiVersion: v1
kind: Secret
metadata:
  name: npmko-credentials
  namespace: npmk-operator-system
type: Opaque
stringData:
  password: "your-npm-password"
```

### DNS Provider Credentials

For Let's Encrypt DNS challenge, create a Secret with your DNS provider credentials:

```yaml
# Cloudflare example
apiVersion: v1
kind: Secret
metadata:
  name: cloudflare-credentials
  namespace: my-app-namespace
type: Opaque
stringData:
  credentials: |
    dns_cloudflare_api_token = your-api-token
```

### Namespace-Specific Override

Create a ConfigMap in your application's namespace to override global settings:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: npmko-config
  namespace: my-app-namespace
data:
  npm-url: "http://different-npm-instance:81"
```

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.19+)
- `kubectl` configured with cluster access
- Nginx Proxy Manager instance accessible from the cluster
- Docker (for building images)

### 1. Deploy the Operator

```bash
# Build and push the operator image
make docker-build IMG=your-registry/npmk-operator:v1.0.0
make docker-push IMG=your-registry/npmk-operator:v1.0.0

# Deploy to cluster
make deploy IMG=your-registry/npmk-operator:v1.0.0
```

### 2. Configure NPM Credentials

```bash
# Edit the sample with your NPM instance details
kubectl apply -f config/samples/configmap.yaml
```

### 3. Create Your First Proxy Host

```bash
kubectl apply -f config/samples/npmko_sample.yaml

# Check status
kubectl get npmko -A
```

### 4. (Optional) Deploy Prometheus Monitoring

If you have Prometheus Operator installed, deploy the ServiceMonitor:

```bash
make deploy-prometheus
```

## How It Works

1. You create an `Npmko` resource in your namespace
2. The operator adds a finalizer and watches for changes
3. It looks up the specified service and its endpoints
4. It reads configuration from ConfigMaps/Secrets (global + namespace override)
5. It authenticates with Nginx Proxy Manager
6. If SSL is enabled, it creates a Let's Encrypt certificate via DNS challenge
7. It creates or updates a proxy host with the discovered endpoint IP/port
8. NPM handles traffic routing with the configured SSL certificate
9. The operator continuously reconciles state every 5 minutes
10. **On deletion**: The operator cleans up the proxy host and certificate in NPM

## Use Cases

- **Multi-tenant Environments**: Each namespace can have its own NPM configuration
- **Dynamic Services**: Automatically update proxy configs when pods change
- **GitOps Workflows**: Manage proxy configs as Kubernetes resources
- **Microservices**: Expose multiple services through a single NPM instance
- **Automatic SSL**: Let's Encrypt certificates without manual intervention

## Status

The operator updates the Npmko status with:

```yaml
status:
  proxyHostId: 42                                       # NPM proxy host ID
  certificateId: 5                                      # NPM Let's Encrypt certificate ID
  forwardHost: "10.42.0.15"                             # Current pod IP
  forwardPort: 8080                                     # Current port
  readyEndpoints: 3                                     # Number of ready pods
  phase: Ready                                          # Pending, Ready, or Error
  message: "Proxy host configured for 10.42.0.15:8080 (3 endpoints ready)"
```

## Troubleshooting

### Check operator logs

```bash
kubectl logs -n npmk-operator-system deployment/npmk-operator-controller-manager
```

### Verify CRD installation

```bash
kubectl get crd npmkos.npmko.io
```

### Check resource status

```bash
kubectl get npmko -A
kubectl describe npmko <name> -n <namespace>
```

## Development

### Development Prerequisites

- Go 1.21+
- Docker
- kubectl
- Access to a Kubernetes cluster

### Local Development

```bash
# Install dependencies
go mod download

# Run locally (requires KUBECONFIG)
export OPERATOR_NAMESPACE=npmk-operator-system
go run main.go

# Run tests
go test ./...
```

### Building

```bash
# Build binary
go build -o bin/manager main.go

# Build container
docker build -t npmk-operator:dev .
```

### Testing

The project includes comprehensive unit tests for all packages:

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run tests with coverage
go test ./... -cover

# Run tests for a specific package
go test ./pkg/npm/... -v
go test ./pkg/config/... -v
go test ./api/v1alpha1/... -v
```

**Test coverage includes:**

- **API types** (`api/v1alpha1/`): Phase constants, spec/status structs, deep copy
- **Config package** (`pkg/config/`): ConfigMap parsing, Secret parsing, validation, merge logic
- **NPM client** (`pkg/npm/`): Login, CRUD operations for proxy hosts, certificate management
- **Controller** (`controllers/`): Endpoint discovery, proxy host sync, status updates, deletion handling

**Current coverage:**

| Package | Coverage |
|---------|----------|
| `controllers` | 70.5% |
| `pkg/config` | 100% |
| `pkg/npm` | 98.4% |
| `pkg/metrics` | 100% |
| `api/v1alpha1` | 72.2% |
| **Total** | **76.4%** |

## API Reference

### NpmkoSpec

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `serviceName` | string | Yes | - | Name of the Kubernetes service to proxy |
| `serviceSelector` | map[string]string | No | - | Alternative label selector for service |
| `domainNames` | []string | Yes | - | List of domain names for this proxy host |
| `forwardScheme` | string | No | `http` | Protocol to use: `http` or `https` |
| `ssl` | SSLConfig | No | - | Let's Encrypt SSL configuration |
| `cachingEnabled` | bool | No | `false` | Enable asset caching |
| `blockExploits` | bool | No | `true` | Block common exploits |
| `allowWebsockets` | bool | No | `true` | Allow websocket upgrades |
| `http2Support` | bool | No | `true` | Enable HTTP/2 protocol |
| `hstsEnabled` | bool | No | `false` | Enable HTTP Strict Transport Security |
| `hstsSubdomains` | bool | No | `false` | Include subdomains in HSTS |
| `advancedConfig` | string | No | - | Custom nginx configuration |

### SSLConfig

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | bool | Yes | - | Enable SSL with Let's Encrypt |
| `forceSSL` | bool | No | `true` | Force HTTPS redirect |
| `dnsChallenge` | DNSChallengeConfig | Yes (if enabled) | - | DNS challenge configuration |

### DNSChallengeConfig

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `provider` | string | Yes | - | DNS provider (cloudflare, route53, digitalocean, powerdns, etc.) |
| `credentialsSecretRef` | SecretReference | Yes | - | Reference to Secret with DNS credentials |
| `propagationSeconds` | int | No | `30` | Seconds to wait for DNS propagation (10-600) |

### NpmkoStatus

| Field | Type | Description |
|-------|------|-------------|
| `proxyHostId` | int | NPM proxy host ID |
| `certificateId` | int | NPM Let's Encrypt certificate ID |
| `forwardHost` | string | Current pod IP being used for forwarding |
| `forwardPort` | int | Current port being used for forwarding |
| `readyEndpoints` | int | Count of ready endpoints available |
| `phase` | string | Current phase: `Pending`, `Ready`, or `Error` |
| `message` | string | Human-readable status message |

## Supported DNS Providers

The operator supports all DNS providers available in Nginx Proxy Manager's Let's Encrypt integration:

- Cloudflare
- AWS Route53
- DigitalOcean
- PowerDNS
- Gandi
- OVH
- And many more...

Credential format follows the certbot DNS plugin requirements for each provider.

## Project Structure

```text
├── api/v1alpha1/          # CRD type definitions
├── controllers/           # Reconciliation logic
├── pkg/
│   ├── config/            # ConfigMap/Secret parsing
│   ├── metrics/           # Prometheus metrics
│   └── npm/               # NPM API client
├── config/
│   ├── crd/               # CRD manifests
│   ├── manager/           # Operator deployment
│   ├── prometheus/        # Prometheus ServiceMonitor
│   ├── rbac/              # RBAC configuration
│   └── samples/           # Example resources
├── charts/npmk-operator/   # Helm chart
└── main.go                # Entry point
```

## Metrics

The operator exposes Prometheus metrics on port `8080` at `/metrics`. For detailed documentation of all available metrics, see [docs/METRICS.md](docs/METRICS.md).

### Quick Metrics Overview

| Metric | Type | Description |
|--------|------|-------------|
| `npmko_reconcile_total` | Counter | Total reconciliations by result |
| `npmko_reconcile_duration_seconds` | Histogram | Reconciliation duration |
| `npmko_resource_info` | Gauge | Resource information labels |
| `npmko_npm_api_requests_total` | Counter | NPM API request count |
| `npmko_npm_api_latency_seconds` | Histogram | NPM API latency |

### Prometheus Integration

Apply the ServiceMonitor for Prometheus Operator:

```bash
kubectl apply -f config/prometheus/
```

## Helm Chart

The operator can be deployed using Helm for easier configuration and upgrades.

### Installation

```bash
# Add the Helm repository (if published)
helm repo add npmk-operator https://your-repo/charts

# Install
helm install npmk-operator npmk-operator/npmk-operator \
  --namespace npmk-operator-system \
  --create-namespace \
  --set npm.url="http://nginx-proxy-manager:81" \
  --set npm.email="admin@example.com" \
  --set npm.password="your-password"
```

### Local Installation

```bash
helm install npmk-operator ./charts/npmk-operator \
  --namespace npmk-operator-system \
  --create-namespace \
  -f my-values.yaml
```

For full Helm chart documentation, see [charts/npmk-operator/README.md](charts/npmk-operator/README.md).

## License

Apache 2.0

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
