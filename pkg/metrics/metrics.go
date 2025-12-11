package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ReconcileTotal counts total reconciliation attempts
	ReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "npmko_reconcile_total",
			Help: "Total number of reconciliations per Npmko resource",
		},
		[]string{"namespace", "name", "result"},
	)

	// ReconcileDuration tracks reconciliation duration
	ReconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "npmko_reconcile_duration_seconds",
			Help:    "Duration of reconciliation in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "name"},
	)

	// ProxyHostsTotal tracks number of proxy hosts by status
	ProxyHostsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "npmko_proxy_hosts_total",
			Help: "Total number of proxy hosts by phase",
		},
		[]string{"phase"},
	)

	// CertificatesTotal tracks number of certificates managed
	CertificatesTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "npmko_certificates_total",
			Help: "Total number of Let's Encrypt certificates managed",
		},
	)

	// NPMAPIRequestsTotal counts NPM API requests
	NPMAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "npmko_npm_api_requests_total",
			Help: "Total number of NPM API requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	// NPMAPILatency tracks NPM API request latency
	NPMAPILatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "npmko_npm_api_latency_seconds",
			Help:    "Latency of NPM API requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	// ResourceInfo provides info about managed resources
	ResourceInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "npmko_resource_info",
			Help: "Information about Npmko resources",
		},
		[]string{"namespace", "name", "phase", "service", "domains"},
	)

	// EndpointChangesTotal counts forward host changes (pod failovers)
	EndpointChangesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "npmko_endpoint_changes_total",
			Help: "Total number of forward host changes due to endpoint unavailability",
		},
		[]string{"namespace", "name"},
	)
)

func init() {
	// Register custom metrics with the controller-runtime metrics registry
	metrics.Registry.MustRegister(
		ReconcileTotal,
		ReconcileDuration,
		ProxyHostsTotal,
		CertificatesTotal,
		NPMAPIRequestsTotal,
		NPMAPILatency,
		ResourceInfo,
		EndpointChangesTotal,
	)
}

// RecordReconcileSuccess records a successful reconciliation
func RecordReconcileSuccess(namespace, name string, duration float64) {
	ReconcileTotal.WithLabelValues(namespace, name, "success").Inc()
	ReconcileDuration.WithLabelValues(namespace, name).Observe(duration)
}

// RecordReconcileError records a failed reconciliation
func RecordReconcileError(namespace, name string, duration float64) {
	ReconcileTotal.WithLabelValues(namespace, name, "error").Inc()
	ReconcileDuration.WithLabelValues(namespace, name).Observe(duration)
}

// RecordNPMAPIRequest records an NPM API request
func RecordNPMAPIRequest(method, endpoint, status string, latency float64) {
	NPMAPIRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
	NPMAPILatency.WithLabelValues(method, endpoint).Observe(latency)
}

// UpdateResourceInfo updates the info metric for a resource
func UpdateResourceInfo(namespace, name, phase, service, domains string) {
	// Clear old values for this resource
	ResourceInfo.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "name": name})
	// Set new value
	ResourceInfo.WithLabelValues(namespace, name, phase, service, domains).Set(1)
}

// DeleteResourceInfo removes the info metric for a deleted resource
func DeleteResourceInfo(namespace, name string) {
	ResourceInfo.DeletePartialMatch(prometheus.Labels{"namespace": namespace, "name": name})
}

// RecordEndpointChange records a forward host change
func RecordEndpointChange(namespace, name string) {
	EndpointChangesTotal.WithLabelValues(namespace, name).Inc()
}
