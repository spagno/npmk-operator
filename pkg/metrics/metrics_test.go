package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordReconcileSuccess(t *testing.T) {
	// Reset the counter for testing
	ReconcileTotal.Reset()
	ReconcileDuration.Reset()

	RecordReconcileSuccess("test-ns", "test-name", 1.5)

	// Check counter was incremented
	count := testutil.ToFloat64(ReconcileTotal.WithLabelValues("test-ns", "test-name", "success"))
	if count != 1 {
		t.Errorf("ReconcileTotal count = %v, want 1", count)
	}
}

func TestRecordReconcileError(t *testing.T) {
	// Reset the counter for testing
	ReconcileTotal.Reset()
	ReconcileDuration.Reset()

	RecordReconcileError("test-ns", "test-name", 0.5)

	// Check counter was incremented
	count := testutil.ToFloat64(ReconcileTotal.WithLabelValues("test-ns", "test-name", "error"))
	if count != 1 {
		t.Errorf("ReconcileTotal count = %v, want 1", count)
	}
}

func TestRecordNPMAPIRequest(t *testing.T) {
	// Reset the counter for testing
	NPMAPIRequestsTotal.Reset()
	NPMAPILatency.Reset()

	RecordNPMAPIRequest("POST", "/api/tokens", "200", 0.25)

	// Check counter was incremented
	count := testutil.ToFloat64(NPMAPIRequestsTotal.WithLabelValues("POST", "/api/tokens", "200"))
	if count != 1 {
		t.Errorf("NPMAPIRequestsTotal count = %v, want 1", count)
	}
}

func TestUpdateResourceInfo(t *testing.T) {
	// Reset the gauge for testing
	ResourceInfo.Reset()

	UpdateResourceInfo("test-ns", "test-name", "Ready", "my-service", "example.com")

	// Check gauge was set
	value := testutil.ToFloat64(ResourceInfo.WithLabelValues("test-ns", "test-name", "Ready", "my-service", "example.com"))
	if value != 1 {
		t.Errorf("ResourceInfo value = %v, want 1", value)
	}
}

func TestDeleteResourceInfo(t *testing.T) {
	// Reset and set a value
	ResourceInfo.Reset()
	UpdateResourceInfo("test-ns", "test-name", "Ready", "my-service", "example.com")

	// Delete and verify
	DeleteResourceInfo("test-ns", "test-name")

	// After deletion, the metric should be gone (0 when queried)
	value := testutil.ToFloat64(ResourceInfo.WithLabelValues("test-ns", "test-name", "Ready", "my-service", "example.com"))
	if value != 0 {
		t.Errorf("ResourceInfo value after delete = %v, want 0", value)
	}
}

func TestRecordEndpointChange(t *testing.T) {
	// Reset the counter for testing
	EndpointChangesTotal.Reset()

	RecordEndpointChange("test-ns", "test-name")

	// Check counter was incremented
	count := testutil.ToFloat64(EndpointChangesTotal.WithLabelValues("test-ns", "test-name"))
	if count != 1 {
		t.Errorf("EndpointChangesTotal count = %v, want 1", count)
	}

	// Record another change
	RecordEndpointChange("test-ns", "test-name")
	count = testutil.ToFloat64(EndpointChangesTotal.WithLabelValues("test-ns", "test-name"))
	if count != 2 {
		t.Errorf("EndpointChangesTotal count = %v, want 2", count)
	}
}

func TestProxyHostsTotal(t *testing.T) {
	// Reset the gauge for testing
	ProxyHostsTotal.Reset()

	// Set values for different phases
	ProxyHostsTotal.WithLabelValues("Ready").Set(5)
	ProxyHostsTotal.WithLabelValues("Pending").Set(2)
	ProxyHostsTotal.WithLabelValues("Error").Set(1)

	// Check values
	if v := testutil.ToFloat64(ProxyHostsTotal.WithLabelValues("Ready")); v != 5 {
		t.Errorf("ProxyHostsTotal Ready = %v, want 5", v)
	}
	if v := testutil.ToFloat64(ProxyHostsTotal.WithLabelValues("Pending")); v != 2 {
		t.Errorf("ProxyHostsTotal Pending = %v, want 2", v)
	}
	if v := testutil.ToFloat64(ProxyHostsTotal.WithLabelValues("Error")); v != 1 {
		t.Errorf("ProxyHostsTotal Error = %v, want 1", v)
	}
}

func TestCertificatesTotal(t *testing.T) {
	// Reset the gauge for testing
	CertificatesTotal.Set(0)

	// Set value
	CertificatesTotal.Set(10)

	// Check value
	if v := testutil.ToFloat64(CertificatesTotal); v != 10 {
		t.Errorf("CertificatesTotal = %v, want 10", v)
	}

	// Increment
	CertificatesTotal.Inc()
	if v := testutil.ToFloat64(CertificatesTotal); v != 11 {
		t.Errorf("CertificatesTotal after Inc = %v, want 11", v)
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are valid prometheus collectors
	collectors := []prometheus.Collector{
		ReconcileTotal,
		ReconcileDuration,
		ProxyHostsTotal,
		CertificatesTotal,
		NPMAPIRequestsTotal,
		NPMAPILatency,
		ResourceInfo,
		EndpointChangesTotal,
	}

	for i, c := range collectors {
		if c == nil {
			t.Errorf("Collector %d is nil", i)
		}
	}
}

func TestReconcileDurationHistogram(t *testing.T) {
	// Reset for testing
	ReconcileDuration.Reset()

	// Record some durations
	ReconcileDuration.WithLabelValues("test-ns", "test-name").Observe(0.1)
	ReconcileDuration.WithLabelValues("test-ns", "test-name").Observe(0.5)
	ReconcileDuration.WithLabelValues("test-ns", "test-name").Observe(1.0)

	// Verify observations were recorded (histogram count should be 3)
	// We use the metric family to check
	ch := make(chan prometheus.Metric, 100)
	ReconcileDuration.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("ReconcileDuration should have collected metrics")
	}
}

func TestNPMAPILatencyHistogram(t *testing.T) {
	// Reset for testing
	NPMAPILatency.Reset()

	// Record some latencies
	NPMAPILatency.WithLabelValues("GET", "/api/nginx/proxy-hosts").Observe(0.05)
	NPMAPILatency.WithLabelValues("POST", "/api/tokens").Observe(0.1)

	// Verify observations were recorded
	ch := make(chan prometheus.Metric, 100)
	NPMAPILatency.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("NPMAPILatency should have collected metrics")
	}
}
