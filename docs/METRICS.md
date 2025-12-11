# NPMK Operator Metrics

The NPMK Operator exposes Prometheus metrics on port `8080` at the `/metrics` endpoint. These metrics provide visibility into operator health, reconciliation performance, and NPM API interactions.

## Available Metrics

### Reconciliation Metrics

#### `npmko_reconcile_total`

- **Type**: Counter
- **Description**: Total number of reconciliations per Npmko resource
- **Labels**:
  - `namespace`: Kubernetes namespace of the resource
  - `name`: Name of the Npmko resource
  - `result`: Result of reconciliation (`success` or `error`)

**Example queries**:

```promql
# Total successful reconciliations
sum(npmko_reconcile_total{result="success"})

# Error rate per resource
rate(npmko_reconcile_total{result="error"}[5m])

# Success/error ratio
sum(rate(npmko_reconcile_total{result="success"}[5m])) / sum(rate(npmko_reconcile_total[5m]))
```

#### `npmko_reconcile_duration_seconds`

- **Type**: Histogram
- **Description**: Duration of reconciliation in seconds
- **Labels**:
  - `namespace`: Kubernetes namespace of the resource
  - `name`: Name of the Npmko resource
- **Buckets**: Default Prometheus buckets (0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10)

**Example queries**:

```promql
# Average reconciliation duration
rate(npmko_reconcile_duration_seconds_sum[5m]) / rate(npmko_reconcile_duration_seconds_count[5m])

# 95th percentile reconciliation time
histogram_quantile(0.95, sum(rate(npmko_reconcile_duration_seconds_bucket[5m])) by (le))

# Slow reconciliations (> 5s)
sum(rate(npmko_reconcile_duration_seconds_bucket{le="5"}[5m])) / sum(rate(npmko_reconcile_duration_seconds_count[5m]))
```

### Resource Metrics

#### `npmko_resource_info`

- **Type**: Gauge (always 1)
- **Description**: Information about Npmko resources with labels
- **Labels**:
  - `namespace`: Kubernetes namespace
  - `name`: Resource name
  - `phase`: Current phase (`Pending`, `Ready`, `Error`)
  - `service`: Target Kubernetes service name
  - `domains`: Comma-separated list of domain names

**Example queries**:

```promql
# Count of resources by phase
count(npmko_resource_info) by (phase)

# List all resources in Error state
npmko_resource_info{phase="Error"}

# Resources per namespace
count(npmko_resource_info) by (namespace)
```

#### `npmko_proxy_hosts_total`

- **Type**: Gauge
- **Description**: Total number of proxy hosts by phase
- **Labels**:
  - `phase`: Phase of the proxy hosts (`Pending`, `Ready`, `Error`)

**Example queries**:

```promql
# Total ready proxy hosts
npmko_proxy_hosts_total{phase="Ready"}

# Total proxy hosts across all phases
sum(npmko_proxy_hosts_total)
```

#### `npmko_certificates_total`

- **Type**: Gauge
- **Description**: Total number of Let's Encrypt certificates managed

**Example queries**:

```promql
# Current certificate count
npmko_certificates_total
```

#### `npmko_endpoint_changes_total`

- **Type**: Counter
- **Description**: Total number of forward host changes due to endpoint unavailability (pod failovers)
- **Labels**:
  - `namespace`: Kubernetes namespace of the resource
  - `name`: Name of the Npmko resource

**Example queries**:

```promql
# Total endpoint changes
sum(npmko_endpoint_changes_total)

# Endpoint changes per resource in the last hour
increase(npmko_endpoint_changes_total[1h])

# Resources with frequent failovers
topk(10, increase(npmko_endpoint_changes_total[24h]))
```

### NPM API Metrics

#### `npmko_npm_api_requests_total`

- **Type**: Counter
- **Description**: Total number of NPM API requests
- **Labels**:
  - `method`: HTTP method (`GET`, `POST`, `PUT`, `DELETE`)
  - `endpoint`: API endpoint path
  - `status`: HTTP status code or `error`

**Example queries**:

```promql
# Total API requests
sum(npmko_npm_api_requests_total)

# API error rate
sum(rate(npmko_npm_api_requests_total{status=~"4..|5.."}[5m])) / sum(rate(npmko_npm_api_requests_total[5m]))

# Requests by endpoint
sum(rate(npmko_npm_api_requests_total[5m])) by (endpoint)
```

#### `npmko_npm_api_latency_seconds`

- **Type**: Histogram
- **Description**: Latency of NPM API requests in seconds
- **Labels**:
  - `method`: HTTP method
  - `endpoint`: API endpoint path
- **Buckets**: Default Prometheus buckets

**Example queries**:

```promql
# Average API latency
rate(npmko_npm_api_latency_seconds_sum[5m]) / rate(npmko_npm_api_latency_seconds_count[5m])

# 99th percentile latency by endpoint
histogram_quantile(0.99, sum(rate(npmko_npm_api_latency_seconds_bucket[5m])) by (le, endpoint))

# Slow API calls (> 2s)
sum(rate(npmko_npm_api_latency_seconds_bucket{le="2"}[5m])) / sum(rate(npmko_npm_api_latency_seconds_count[5m]))
```

## Prometheus ServiceMonitor

For Prometheus Operator users, apply the ServiceMonitor:

```bash
kubectl apply -f config/prometheus/
```

This creates:

- A `Service` exposing the metrics port
- A `ServiceMonitor` that tells Prometheus how to scrape the operator

### ServiceMonitor Configuration

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: npmk-operator
  labels:
    app: npmk-operator
spec:
  selector:
    matchLabels:
      app: npmk-operator
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
  namespaceSelector:
    matchNames:
    - npmk-operator-system
```

## Grafana Dashboard

### Sample Dashboard JSON

You can import this dashboard into Grafana for visualizing NPMK Operator metrics:

```json
{
  "title": "NPMK Operator",
  "panels": [
    {
      "title": "Reconciliation Rate",
      "type": "graph",
      "targets": [
        {
          "expr": "sum(rate(npmko_reconcile_total[5m])) by (result)",
          "legendFormat": "{{result}}"
        }
      ]
    },
    {
      "title": "Resources by Phase",
      "type": "piechart",
      "targets": [
        {
          "expr": "count(npmko_resource_info) by (phase)",
          "legendFormat": "{{phase}}"
        }
      ]
    },
    {
      "title": "Reconciliation Duration P95",
      "type": "gauge",
      "targets": [
        {
          "expr": "histogram_quantile(0.95, sum(rate(npmko_reconcile_duration_seconds_bucket[5m])) by (le))"
        }
      ]
    },
    {
      "title": "NPM API Latency",
      "type": "graph",
      "targets": [
        {
          "expr": "histogram_quantile(0.95, sum(rate(npmko_npm_api_latency_seconds_bucket[5m])) by (le, endpoint))",
          "legendFormat": "{{endpoint}}"
        }
      ]
    }
  ]
}
```

## Alerting Rules

Example Prometheus alerting rules for the NPMK Operator:

```yaml
groups:
- name: npmk-operator
  rules:
  - alert: NpmOperatorReconcileErrors
    expr: sum(rate(npmko_reconcile_total{result="error"}[5m])) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: High reconciliation error rate
      description: "NPMK Operator is experiencing {{ $value | humanize }} errors/s"

  - alert: NpmOperatorResourcesInError
    expr: count(npmko_resource_info{phase="Error"}) > 0
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: Npmko resources in Error state
      description: "{{ $value }} resources are in Error state"

  - alert: NpmOperatorSlowReconciliation
    expr: histogram_quantile(0.95, sum(rate(npmko_reconcile_duration_seconds_bucket[5m])) by (le)) > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: Slow reconciliation detected
      description: "95th percentile reconciliation time is {{ $value | humanize }}s"

  - alert: NpmAPIHighLatency
    expr: histogram_quantile(0.99, sum(rate(npmko_npm_api_latency_seconds_bucket[5m])) by (le)) > 5
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: High NPM API latency
      description: "99th percentile API latency is {{ $value | humanize }}s"

  - alert: NpmAPIErrors
    expr: sum(rate(npmko_npm_api_requests_total{status=~"5.."}[5m])) > 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: NPM API server errors
      description: "NPM API is returning 5xx errors"
```

## Scrape Configuration (without Prometheus Operator)

If you're not using Prometheus Operator, add this to your Prometheus config:

```yaml
scrape_configs:
  - job_name: 'npmk-operator'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - npmk-operator-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        regex: npmk-operator
        action: keep
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        regex: metrics
        action: keep
```
