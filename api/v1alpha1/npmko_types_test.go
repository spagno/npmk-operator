package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPhaseConstants(t *testing.T) {
	// Ensure phase constants have expected values
	tests := []struct {
		phase string
		want  string
	}{
		{PhasePending, "Pending"},
		{PhaseReady, "Ready"},
		{PhaseError, "Error"},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			if tt.phase != tt.want {
				t.Errorf("Phase constant = %v, want %v", tt.phase, tt.want)
			}
		})
	}
}

func TestNpmkoSpec(t *testing.T) {
	spec := NpmkoSpec{
		ServiceName:     "my-service",
		DomainNames:     []string{"example.com", "www.example.com"},
		ForwardScheme:   "http",
		CachingEnabled:  false,
		BlockExploits:   true,
		AllowWebsockets: true,
		HTTP2Support:    true,
		HSTSEnabled:     false,
		HSTSSubdomains:  false,
	}

	if spec.ServiceName != "my-service" {
		t.Errorf("ServiceName = %v, want %v", spec.ServiceName, "my-service")
	}
	if len(spec.DomainNames) != 2 {
		t.Errorf("DomainNames length = %v, want %v", len(spec.DomainNames), 2)
	}
}

func TestNpmkoSpecWithAllFields(t *testing.T) {
	propagation := 60
	spec := NpmkoSpec{
		ServiceName:     "my-service",
		ServiceSelector: map[string]string{"app": "myapp"},
		DomainNames:     []string{"example.com"},
		ForwardScheme:   "https",
		CachingEnabled:  true,
		BlockExploits:   true,
		AllowWebsockets: true,
		HTTP2Support:    true,
		HSTSEnabled:     true,
		HSTSSubdomains:  true,
		AdvancedConfig:  "proxy_read_timeout 300;",
		SSL: &SSLConfig{
			Enabled:  true,
			ForceSSL: true,
			DNSChallenge: DNSChallengeConfig{
				Provider: "cloudflare",
				CredentialsSecretRef: SecretReference{
					Name: "cf-creds",
					Key:  "api-token",
				},
				PropagationSeconds: &propagation,
			},
		},
	}

	if spec.ServiceSelector["app"] != "myapp" {
		t.Error("ServiceSelector should contain app=myapp")
	}
	if spec.ForwardScheme != "https" {
		t.Errorf("ForwardScheme = %v, want https", spec.ForwardScheme)
	}
	if !spec.CachingEnabled {
		t.Error("CachingEnabled should be true")
	}
	if !spec.HSTSEnabled {
		t.Error("HSTSEnabled should be true")
	}
	if !spec.HSTSSubdomains {
		t.Error("HSTSSubdomains should be true")
	}
	if spec.AdvancedConfig == "" {
		t.Error("AdvancedConfig should not be empty")
	}
	if spec.SSL == nil {
		t.Error("SSL should not be nil")
	} else if spec.SSL.DNSChallenge.PropagationSeconds == nil || *spec.SSL.DNSChallenge.PropagationSeconds != 60 {
		t.Error("PropagationSeconds should be 60")
	}
}

func TestNpmkoStatus(t *testing.T) {
	status := NpmkoStatus{
		ProxyHostId:    42,
		CertificateId:  123,
		ForwardHost:    "10.0.0.1",
		ForwardPort:    8080,
		ReadyEndpoints: 3,
		Phase:          PhaseReady,
		Message:        "Proxy host configured",
	}

	if status.ProxyHostId != 42 {
		t.Errorf("ProxyHostId = %v, want %v", status.ProxyHostId, 42)
	}
	if status.CertificateId != 123 {
		t.Errorf("CertificateId = %v, want %v", status.CertificateId, 123)
	}
	if status.ForwardHost != "10.0.0.1" {
		t.Errorf("ForwardHost = %v, want %v", status.ForwardHost, "10.0.0.1")
	}
	if status.ForwardPort != 8080 {
		t.Errorf("ForwardPort = %v, want %v", status.ForwardPort, 8080)
	}
	if status.ReadyEndpoints != 3 {
		t.Errorf("ReadyEndpoints = %v, want %v", status.ReadyEndpoints, 3)
	}
	if status.Phase != PhaseReady {
		t.Errorf("Phase = %v, want %v", status.Phase, PhaseReady)
	}
}

func TestNpmkoStatusAllPhases(t *testing.T) {
	tests := []struct {
		phase   string
		message string
	}{
		{PhasePending, "Waiting for service"},
		{PhaseReady, "Proxy host configured successfully"},
		{PhaseError, "Failed to create proxy host"},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			status := NpmkoStatus{
				Phase:   tt.phase,
				Message: tt.message,
			}
			if status.Phase != tt.phase {
				t.Errorf("Phase = %v, want %v", status.Phase, tt.phase)
			}
			if status.Message != tt.message {
				t.Errorf("Message = %v, want %v", status.Message, tt.message)
			}
		})
	}
}

func TestSSLConfig(t *testing.T) {
	propagation := 30
	ssl := SSLConfig{
		Enabled:  true,
		ForceSSL: true,
		DNSChallenge: DNSChallengeConfig{
			Provider: "cloudflare",
			CredentialsSecretRef: SecretReference{
				Name: "cloudflare-creds",
				Key:  "credentials",
			},
			PropagationSeconds: &propagation,
		},
	}

	if !ssl.Enabled {
		t.Error("SSL.Enabled should be true")
	}
	if !ssl.ForceSSL {
		t.Error("SSL.ForceSSL should be true")
	}
	if ssl.DNSChallenge.Provider != "cloudflare" {
		t.Errorf("DNSChallenge.Provider = %v, want %v", ssl.DNSChallenge.Provider, "cloudflare")
	}
	if ssl.DNSChallenge.CredentialsSecretRef.Name != "cloudflare-creds" {
		t.Errorf("CredentialsSecretRef.Name = %v, want %v", ssl.DNSChallenge.CredentialsSecretRef.Name, "cloudflare-creds")
	}
}

func TestSSLConfigDNSProviders(t *testing.T) {
	providers := []string{"cloudflare", "route53", "digitalocean", "powerdns", "acmedns"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			ssl := SSLConfig{
				Enabled: true,
				DNSChallenge: DNSChallengeConfig{
					Provider: provider,
					CredentialsSecretRef: SecretReference{
						Name: provider + "-creds",
						Key:  "credentials",
					},
				},
			}
			if ssl.DNSChallenge.Provider != provider {
				t.Errorf("Provider = %v, want %v", ssl.DNSChallenge.Provider, provider)
			}
		})
	}
}

func TestSecretReference(t *testing.T) {
	tests := []struct {
		name string
		ref  SecretReference
	}{
		{
			name: "with explicit key",
			ref:  SecretReference{Name: "my-secret", Key: "api-token"},
		},
		{
			name: "with default key",
			ref:  SecretReference{Name: "my-secret", Key: "credentials"},
		},
		{
			name: "empty key",
			ref:  SecretReference{Name: "my-secret", Key: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ref.Name != "my-secret" {
				t.Errorf("Name = %v, want my-secret", tt.ref.Name)
			}
		})
	}
}

func TestNpmkoDeepCopy(t *testing.T) {
	original := &Npmko{
		Spec: NpmkoSpec{
			ServiceName:   "test-service",
			DomainNames:   []string{"test.com"},
			ForwardScheme: "http",
		},
		Status: NpmkoStatus{
			ProxyHostId: 1,
			Phase:       PhaseReady,
		},
	}

	// Test DeepCopy
	copied := original.DeepCopy()
	if copied == original {
		t.Error("DeepCopy should return a new object")
	}
	if copied.Spec.ServiceName != original.Spec.ServiceName {
		t.Error("DeepCopy should copy ServiceName")
	}

	// Modify copy shouldn't affect original
	copied.Spec.ServiceName = "modified"
	if original.Spec.ServiceName == "modified" {
		t.Error("Modifying copy should not affect original")
	}
}

func TestNpmkoDeepCopyInto(t *testing.T) {
	original := &Npmko{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: NpmkoSpec{
			ServiceName:   "test-service",
			DomainNames:   []string{"test.com", "www.test.com"},
			ForwardScheme: "http",
			BlockExploits: true,
		},
		Status: NpmkoStatus{
			ProxyHostId:    42,
			ForwardHost:    "10.0.0.1",
			ReadyEndpoints: 2,
			Phase:          PhaseReady,
		},
	}

	copied := &Npmko{}
	original.DeepCopyInto(copied)

	if copied.Name != original.Name {
		t.Error("DeepCopyInto should copy Name")
	}
	if copied.Namespace != original.Namespace {
		t.Error("DeepCopyInto should copy Namespace")
	}
	if copied.Spec.ServiceName != original.Spec.ServiceName {
		t.Error("DeepCopyInto should copy Spec.ServiceName")
	}
	if len(copied.Spec.DomainNames) != len(original.Spec.DomainNames) {
		t.Error("DeepCopyInto should copy Spec.DomainNames")
	}
	if copied.Status.ProxyHostId != original.Status.ProxyHostId {
		t.Error("DeepCopyInto should copy Status.ProxyHostId")
	}

	// Modify copy shouldn't affect original
	copied.Spec.DomainNames[0] = "modified.com"
	if original.Spec.DomainNames[0] == "modified.com" {
		t.Error("Modifying copy should not affect original")
	}
}

func TestNpmkoDeepCopyObject(t *testing.T) {
	original := &Npmko{
		Spec: NpmkoSpec{
			ServiceName: "test-service",
			DomainNames: []string{"test.com"},
		},
	}

	copiedObj := original.DeepCopyObject()
	if copiedObj == nil {
		t.Fatal("DeepCopyObject should not return nil")
	}

	copied, ok := copiedObj.(*Npmko)
	if !ok {
		t.Fatal("DeepCopyObject should return *Npmko")
	}
	if copied.Spec.ServiceName != original.Spec.ServiceName {
		t.Error("DeepCopyObject should copy ServiceName")
	}
}

func TestNpmkoListDeepCopy(t *testing.T) {
	original := &NpmkoList{
		Items: []Npmko{
			{
				Spec: NpmkoSpec{
					ServiceName: "service-1",
					DomainNames: []string{"a.com"},
				},
			},
			{
				Spec: NpmkoSpec{
					ServiceName: "service-2",
					DomainNames: []string{"b.com"},
				},
			},
		},
	}

	copied := original.DeepCopy()
	if copied == original {
		t.Error("DeepCopy should return a new object")
	}
	if len(copied.Items) != len(original.Items) {
		t.Error("DeepCopy should copy all items")
	}

	// Modify copy shouldn't affect original
	copied.Items[0].Spec.ServiceName = "modified"
	if original.Items[0].Spec.ServiceName == "modified" {
		t.Error("Modifying copy should not affect original")
	}
}

func TestNpmkoListDeepCopyObject(t *testing.T) {
	original := &NpmkoList{
		Items: []Npmko{
			{Spec: NpmkoSpec{ServiceName: "test"}},
		},
	}

	copiedObj := original.DeepCopyObject()
	if copiedObj == nil {
		t.Fatal("DeepCopyObject should not return nil")
	}

	copied, ok := copiedObj.(*NpmkoList)
	if !ok {
		t.Fatal("DeepCopyObject should return *NpmkoList")
	}
	if len(copied.Items) != 1 {
		t.Error("DeepCopyObject should copy all items")
	}
}

func TestNpmkoSpecDeepCopy(t *testing.T) {
	propagation := 30
	original := NpmkoSpec{
		ServiceName:     "test-service",
		ServiceSelector: map[string]string{"app": "test"},
		DomainNames:     []string{"a.com", "b.com"},
		SSL: &SSLConfig{
			Enabled: true,
			DNSChallenge: DNSChallengeConfig{
				Provider:           "cloudflare",
				PropagationSeconds: &propagation,
			},
		},
	}

	copied := original.DeepCopy()
	if copied == nil {
		t.Fatal("DeepCopy should not return nil")
		return
	}

	// Modify copy shouldn't affect original
	copied.ServiceSelector["app"] = "modified"
	if original.ServiceSelector["app"] == "modified" {
		t.Error("Modifying copy should not affect original ServiceSelector")
	}

	copied.DomainNames[0] = "modified.com"
	if original.DomainNames[0] == "modified.com" {
		t.Error("Modifying copy should not affect original DomainNames")
	}

	if copied.SSL == nil || copied.SSL.DNSChallenge.PropagationSeconds == nil {
		t.Fatal("DeepCopy should copy SSL config with PropagationSeconds")
		return
	}
	*copied.SSL.DNSChallenge.PropagationSeconds = 60
	if *original.SSL.DNSChallenge.PropagationSeconds != 30 {
		t.Error("Modifying copy should not affect original PropagationSeconds")
	}
}

func TestNpmkoStatusDeepCopy(t *testing.T) {
	original := NpmkoStatus{
		ProxyHostId:    42,
		CertificateId:  123,
		ForwardHost:    "10.0.0.1",
		ForwardPort:    8080,
		ReadyEndpoints: 3,
		Phase:          PhaseReady,
		Message:        "OK",
	}

	copied := original.DeepCopy()
	if copied == nil {
		t.Fatal("DeepCopy should not return nil")
		return
	}
	if copied.ProxyHostId != original.ProxyHostId {
		t.Error("DeepCopy should copy ProxyHostId")
	}
	if copied.Phase != original.Phase {
		t.Error("DeepCopy should copy Phase")
	}
}
