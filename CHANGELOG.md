# Changelog

All notable changes to the NPMK Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1] - 2025-12-12

### Added

- Comprehensive unit tests for controller package (`controllers/npmko_controller_test.go`)
  - Tests for `EndpointInfo` struct
  - Tests for `buildProxyHost` method
  - Tests for `discoverEndpoints` with various endpoint scenarios
  - Tests for `syncProxyHost` create and update operations
  - Tests for `getDNSCredentials` with explicit and default keys
  - Tests for `loadConfig` with global and namespace-local overrides
  - Tests for `handleDeletion` with finalizer cleanup
  - Tests for `updateStatusError`, `updateStatusPending`, `updateStatusReady`
  - Tests for `ensureCertificate` with existing, found, and new certificates
  - Tests for reconciliation error scenarios
- Extended NPM client tests (`pkg/npm/client_test.go`)
  - JSON decode error handling tests for all API methods
  - Network error tests for all client operations
  - Tests for nil propagation seconds in certificate creation
  - Struct field validation tests for `ProxyHost`, `CertificateRequest`, `CertificateResponse`
  - `readErrorBody` function tests
  - Client timeout verification tests
- Extended config package tests (`pkg/config/config_test.go`)
  - `ConfigError.Error()` method tests
  - Config constants validation tests
  - Config struct field tests
  - Edge case tests for nil data in ConfigMap and Secret parsing
  - Empty config merge tests

### Changed

- Test coverage significantly improved:
  - `controllers/`: 0% → 70.5%
  - `pkg/npm/`: 88.5% → 98.4%
  - `pkg/config/`: ~85% → 100%
  - `pkg/metrics/`: 100%
  - Overall: ~45% → 76.4%
- Updated CRD printer columns to show `Service`, `SSL`, `Forward Host`, and `Endpoints` in `kubectl get` output
- Updated README with current test coverage statistics

### Dependencies

- Updated `golang.org/x/net` from v0.43.0 to v0.47.0

## [0.1.0] - 2025-12-11

### Added

- Initial release of NPMK Operator
- Complete Kubernetes operator for Nginx Proxy Manager
- Custom Resource Definition (CRD) for `Npmko` resources
- Let's Encrypt SSL certificates via DNS challenge (Cloudflare, Route53, DigitalOcean, PowerDNS, etc.)
- EndpointSlice API for service endpoint discovery
- Forward host tracking for multi-pod stability (keeps using same pod IP when available)
- Finalizer for cleanup on resource deletion
- Prometheus metrics for reconciliation, NPM API calls, and resource info
- `npmko_endpoint_changes_total` metric for tracking pod failovers
- "Forward Host" and "Endpoints" printer columns in CRD for better `kubectl get` output
- ServiceMonitor for Prometheus Operator integration
- Helm chart with full configuration options
- Health probes (liveness, readiness) on port 8081
- Leader election support for HA deployments
- Namespace-local config override pattern
- Support for HTTP/2, HSTS, websockets, caching, and custom nginx configs
- NPM API client for proxy host and certificate management
- Unit tests for config, npm client, and API types packages

### Technical Details

- Built with controller-runtime v0.16.x
- Uses distroless container base image
- RBAC includes EndpointSlices, finalizers, and leader election leases
- Reconcile intervals: 5 minutes (success), 30 seconds (error/pending)
