# Changelog

All notable changes to the NPMK Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
