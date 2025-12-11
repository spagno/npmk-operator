# NPMK Operator - AI Coding Instructions

## Project Overview
This is a **Kubernetes operator** built with [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) that manages Nginx Proxy Manager (NPM) proxy hosts based on `Npmko` custom resources. It watches K8s services/endpoints and automatically configures NPM routing with Let's Encrypt SSL certificates via DNS challenge.

## Architecture

### Core Data Flow
1. User creates `Npmko` CR → Controller reconciles
2. Controller reads ConfigMap (`npmko-config`) and Secret (`npmko-credentials`) for NPM credentials
3. Controller discovers target Service endpoints via EndpointSlice API
4. If SSL enabled: Controller reads DNS credentials from Secret and creates Let's Encrypt certificate via NPM API
5. Controller calls NPM REST API to create/update proxy hosts
6. Status updated with `ProxyHostId`, `CertificateId`, `ForwardHost`, `ForwardPort`, `ReadyEndpoints`, `Phase`, and `Message`
7. If pod changes, controller detects the forward host is no longer available and updates to a new pod IP

### Key Components
| Directory | Purpose |
|-----------|---------|
| `api/v1alpha1/` | CRD types with kubebuilder validation markers |
| `controllers/` | Reconciliation logic in `NpmkoReconciler` |
| `pkg/npm/` | HTTP client for NPM API with helper methods and timeout |
| `pkg/config/` | ConfigMap/Secret parsing with validation and merge pattern |
| `pkg/metrics/` | Prometheus metrics registration and helpers |
| `config/` | Kustomize manifests for deployment |
| `charts/npmk-operator/` | Helm chart for deployment |

## SSL/Let's Encrypt Configuration
Certificates are managed automatically via NPM's Let's Encrypt DNS challenge. DNS provider credentials are stored in Kubernetes Secrets:

```yaml
ssl:
  enabled: true
  forceSSL: true
  dnsChallenge:
    provider: cloudflare  # cloudflare, route53, digitalocean, powerdns, etc.
    credentialsSecretRef:
      name: cloudflare-dns-credentials
      key: credentials
    propagationSeconds: 30
```

The Secret format matches certbot DNS plugin requirements (see `config/samples/`).

## Development Commands

```bash
# Build and push container image
make docker-build IMG=<registry>/npmk-operator:tag
make docker-push IMG=<registry>/npmk-operator:tag

# Deploy/undeploy to cluster
make deploy
make undeploy
```

## Code Patterns

### Phase Constants
Use defined constants instead of magic strings:
```go
npmko.Status.Phase = npmkov1alpha1.PhasePending  // "Pending"
npmko.Status.Phase = npmkov1alpha1.PhaseReady    // "Ready"
npmko.Status.Phase = npmkov1alpha1.PhaseError    // "Error"
```

### Configuration Constants
Use constants from `pkg/config`:
```go
config.ConfigMapName         // "npmko-config"
config.CredentialsSecretName // "npmko-credentials"
config.GetOperatorNamespace() // reads from OPERATOR_NAMESPACE env
```

### RBAC Annotations
Controller permissions are declared via kubebuilder markers in `controllers/npmko_controller.go`:
```go
// +kubebuilder:rbac:groups=npmko.io,resources=npmkos,verbs=get;list;watch;...
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
```

### Reconciliation Pattern
- Requeue on transient errors: `requeueOnError = 30 * time.Second`
- Periodic re-sync: `requeueOnSuccess = 5 * time.Minute`
- Status updates use `r.Status().Update()` (not `r.Update()`)
- Use helper methods: `updateStatusError()`, `updateStatusPending()`, `updateStatusReady()`

### Finalizer Pattern
The operator uses a finalizer (`npmko.io/finalizer`) to clean up NPM resources when the CR is deleted:
- Finalizer added on first reconciliation
- On deletion: proxy host deleted first, then certificate
- Uses `controllerutil.ContainsFinalizer()`, `AddFinalizer()`, `RemoveFinalizer()`
- RBAC requires `npmkos/finalizers` update permission

### Config Override Pattern
Global config in operator namespace is merged with namespace-local config. Password can be in ConfigMap or Secret (Secret preferred). See `loadConfig()` in controller.

### NPM API Client
The `pkg/npm/client.go` wraps NPM REST API with:
- `httpClient` with 30s timeout
- `doRequest()` helper for all HTTP calls
- `readErrorBody()` for error details
- Methods: `Login()`, `CreateProxyHost()`, `UpdateProxyHost()`, `CreateLetsEncryptCertificate()`, `GetCertificate()`

### Kubebuilder Validation
Type definitions use kubebuilder markers for CRD validation:
```go
// +kubebuilder:validation:Required
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:Enum=http;https
// +kubebuilder:default:=true
```

## Type Definitions
When modifying `api/v1alpha1/npmko_types.go`:
1. Add JSON tags with `json:"fieldName,omitempty"` pattern
2. Add kubebuilder validation markers
3. Update CRD YAML in `config/crd/bases/`
4. Run `go generate ./...` to regenerate `zz_generated.deepcopy.go`

## External Dependencies
- **Nginx Proxy Manager**: REST API at `/api/tokens`, `/api/nginx/proxy-hosts`, `/api/nginx/certificates`
- **controller-runtime v0.16.x**: Manager, client, reconciler patterns (uses `Metrics: metricsserver.Options{}`)
- Uses distroless container base image for security
