# NPMK Operator Helm Chart

A Helm chart for deploying the NPMK Operator - a Kubernetes operator that automatically manages Nginx Proxy Manager proxy hosts with Let's Encrypt SSL certificates via DNS challenge.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- Nginx Proxy Manager instance accessible from the cluster

## Installation

### Quick Start

```bash
helm install npmk-operator ./charts/npmk-operator \
  --namespace npmk-operator-system \
  --create-namespace \
  --set npm.url="http://nginx-proxy-manager:81" \
  --set npm.email="admin@example.com" \
  --set npm.password="your-password"
```

### Using a Values File

Create a `values.yaml` file:

```yaml
npm:
  url: "http://nginx-proxy-manager:81"
  email: "admin@example.com"
  password: "your-password"

metrics:
  serviceMonitor:
    enabled: true
```

Then install:

```bash
helm install npmk-operator ./charts/npmk-operator \
  --namespace npmk-operator-system \
  --create-namespace \
  -f values.yaml
```

### Using an Existing Secret

If you already have a Secret with the NPM password:

```bash
helm install npmk-operator ./charts/npmk-operator \
  --namespace npmk-operator-system \
  --create-namespace \
  --set npm.url="http://nginx-proxy-manager:81" \
  --set npm.email="admin@example.com" \
  --set npm.existingSecret="my-npm-secret" \
  --set npm.existingSecretKey="password"
```

## Uninstallation

```bash
helm uninstall npmk-operator -n npmk-operator-system
```

**Note**: The CRD and any Npmko resources will remain. To fully clean up:

```bash
kubectl delete crd npmkos.npmko.io
```

## Configuration

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `nameOverride` | Override chart name | `""` |
| `fullnameOverride` | Override full name | `""` |

### Image Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository (required) | `""` |
| `image.pullPolicy` | Image pull policy | `Always` |
| `image.tag` | Image tag | `""` (defaults to Chart appVersion) |
| `imagePullSecrets` | Image pull secrets | `[]` |

### NPM Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `npm.url` | NPM API URL (required) | `""` |
| `npm.email` | NPM admin email (required) | `""` |
| `npm.password` | NPM admin password | `""` |
| `npm.existingSecret` | Use existing secret for password | `""` |
| `npm.existingSecretKey` | Key in existing secret | `"password"` |

### Operator Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.leaderElect` | Enable leader election | `true` |
| `operator.metricsAddr` | Metrics bind address | `":8080"` |
| `operator.healthProbeAddr` | Health probe bind address | `":8081"` |
| `operator.logLevel` | Log level | `"info"` |

### Metrics Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.port` | Metrics service port | `8080` |
| `metrics.serviceMonitor.enabled` | Create ServiceMonitor | `false` |
| `metrics.serviceMonitor.interval` | Scrape interval | `"30s"` |
| `metrics.serviceMonitor.labels` | Additional labels | `{}` |
| `metrics.serviceMonitor.namespace` | ServiceMonitor namespace | `""` |

### Health Probes

| Parameter | Description | Default |
|-----------|-------------|---------|
| `probes.liveness.enabled` | Enable liveness probe | `true` |
| `probes.liveness.initialDelaySeconds` | Initial delay | `15` |
| `probes.liveness.periodSeconds` | Period | `20` |
| `probes.readiness.enabled` | Enable readiness probe | `true` |
| `probes.readiness.initialDelaySeconds` | Initial delay | `5` |
| `probes.readiness.periodSeconds` | Period | `10` |

### RBAC Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name | `""` |

### Resource Management

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |

### Security Context

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podSecurityContext` | Pod security context | `{}` |
| `securityContext.allowPrivilegeEscalation` | Allow privilege escalation | `false` |
| `securityContext.runAsNonRoot` | Run as non-root | `true` |
| `securityContext.runAsUser` | Run as user ID | `65532` |

### Scheduling

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity rules | `{}` |

## Examples

### Production Deployment

```yaml
replicaCount: 1

image:
  repository: ghcr.io/spagno/npmk-operator  # Set to your registry
  pullPolicy: Always

npm:
  url: "http://nginx-proxy-manager.npm.svc.cluster.local:81"
  email: "admin@example.com"
  existingSecret: "npm-admin-credentials"
  existingSecretKey: "password"

metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    labels:
      release: prometheus

resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/os
          operator: In
          values:
          - linux
```

### Development Deployment

```yaml
operator:
  leaderElect: false
  logLevel: debug

npm:
  url: "http://npm-dev:81"
  email: "dev@example.com"
  password: "devpassword"

resources:
  limits:
    cpu: 200m
    memory: 64Mi
  requests:
    cpu: 10m
    memory: 32Mi
```

## Creating Npmko Resources

After installing the operator, create proxy hosts using the `Npmko` CRD:

```yaml
apiVersion: npmko.io/v1alpha1
kind: Npmko
metadata:
  name: my-app
  namespace: production
spec:
  serviceName: my-app-service
  domainNames:
    - myapp.example.com
  ssl:
    enabled: true
    forceSSL: true
    dnsChallenge:
      provider: cloudflare
      credentialsSecretRef:
        name: cloudflare-credentials
        key: credentials
```

## Upgrading

```bash
helm upgrade npmk-operator ./charts/npmk-operator \
  --namespace npmk-operator-system \
  -f values.yaml
```

## Troubleshooting

### Check operator logs

```bash
kubectl logs -n npmk-operator-system deployment/npmk-operator
```

### Check Npmko status

```bash
kubectl get npmko -A
kubectl describe npmko <name> -n <namespace>
```

### Verify metrics

```bash
kubectl port-forward -n npmk-operator-system svc/npmk-operator-metrics 8080:8080
curl http://localhost:8080/metrics
```

## License

Apache 2.0
