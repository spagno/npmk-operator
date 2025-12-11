package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Phase constants for Npmko status
const (
	PhasePending = "Pending"
	PhaseReady   = "Ready"
	PhaseError   = "Error"
)

// DNSChallengeConfig defines DNS provider settings for Let's Encrypt
type DNSChallengeConfig struct {
	// Provider is the DNS provider name (e.g., "cloudflare", "route53", "digitalocean", "powerdns")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// CredentialsSecretRef references a Secret containing provider credentials
	// The secret should contain the credentials in the format expected by certbot
	// Example for Cloudflare: dns_cloudflare_api_token = YOUR_TOKEN
	// +kubebuilder:validation:Required
	CredentialsSecretRef SecretReference `json:"credentialsSecretRef"`

	// PropagationSeconds is the time to wait for DNS propagation
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=3600
	// +kubebuilder:default=30
	// +optional
	PropagationSeconds *int `json:"propagationSeconds,omitempty"`
}

// SecretReference references a Secret in the same namespace
type SecretReference struct {
	// Name of the Secret
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key in the Secret containing the credentials
	// +kubebuilder:default="credentials"
	// +optional
	Key string `json:"key,omitempty"`
}

// SSLConfig defines Let's Encrypt certificate configuration
type SSLConfig struct {
	// Enabled enables SSL with Let's Encrypt
	// +kubebuilder:default=false
	Enabled bool `json:"enabled"`

	// ForceSSL redirects HTTP to HTTPS
	// +kubebuilder:default=false
	// +optional
	ForceSSL bool `json:"forceSSL,omitempty"`

	// DNSChallenge configuration for Let's Encrypt
	// +kubebuilder:validation:Required
	DNSChallenge DNSChallengeConfig `json:"dnsChallenge"`
}

// NpmkoSpec defines the desired state of Npmko
type NpmkoSpec struct {
	// ServiceName is the target Kubernetes service
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServiceName string `json:"serviceName"`

	// ServiceSelector for advanced service selection
	// +optional
	ServiceSelector map[string]string `json:"serviceSelector,omitempty"`

	// DomainNames is the list of domain names for the proxy host
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	DomainNames []string `json:"domainNames"`

	// ForwardScheme is http or https
	// +kubebuilder:validation:Enum=http;https
	// +kubebuilder:default="http"
	// +optional
	ForwardScheme string `json:"forwardScheme,omitempty"`

	// CachingEnabled enables caching
	// +kubebuilder:default=false
	// +optional
	CachingEnabled bool `json:"cachingEnabled,omitempty"`

	// BlockExploits blocks common exploits
	// +kubebuilder:default=true
	// +optional
	BlockExploits bool `json:"blockExploits,omitempty"`

	// AllowWebsockets allows websocket upgrade
	// +kubebuilder:default=true
	// +optional
	AllowWebsockets bool `json:"allowWebsockets,omitempty"`

	// SSL configuration for Let's Encrypt certificates via DNS challenge
	// +optional
	SSL *SSLConfig `json:"ssl,omitempty"`

	// HTTP2Support enables HTTP/2
	// +kubebuilder:default=true
	// +optional
	HTTP2Support bool `json:"http2Support,omitempty"`

	// HSTSEnabled enables HSTS
	// +kubebuilder:default=false
	// +optional
	HSTSEnabled bool `json:"hstsEnabled,omitempty"`

	// HSTSSubdomains includes subdomains in HSTS
	// +kubebuilder:default=false
	// +optional
	HSTSSubdomains bool `json:"hstsSubdomains,omitempty"`

	// AdvancedConfig for custom Nginx configuration
	// +optional
	AdvancedConfig string `json:"advancedConfig,omitempty"`
}

// NpmkoStatus defines the observed state of Npmko
type NpmkoStatus struct {
	// ProxyHostId is the NPM proxy host ID
	// +optional
	ProxyHostId int `json:"proxyHostId,omitempty"`

	// CertificateId is the NPM certificate ID (auto-managed)
	// +optional
	CertificateId int `json:"certificateId,omitempty"`

	// ForwardHost is the current pod IP being used for forwarding
	// +optional
	ForwardHost string `json:"forwardHost,omitempty"`

	// ForwardPort is the current port being used for forwarding
	// +optional
	ForwardPort int `json:"forwardPort,omitempty"`

	// ReadyEndpoints is the count of ready endpoints available
	// +optional
	ReadyEndpoints int `json:"readyEndpoints,omitempty"`

	// Phase indicates the current state (Pending, Ready, Error)
	// +kubebuilder:validation:Enum=Pending;Ready;Error
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides additional status information
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="ProxyHostId",type="integer",JSONPath=".status.proxyHostId"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Npmko struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NpmkoSpec   `json:"spec,omitempty"`
	Status NpmkoStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type NpmkoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Npmko `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Npmko{}, &NpmkoList{})
}
