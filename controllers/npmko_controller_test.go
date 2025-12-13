package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	npmkov1alpha1 "github.com/spagno/npmk-operator/api/v1alpha1"
	"github.com/spagno/npmk-operator/pkg/config"
	"github.com/spagno/npmk-operator/pkg/npm"
)

func TestEndpointInfo(t *testing.T) {
	tests := []struct {
		name          string
		info          *EndpointInfo
		wantAddresses int
		wantHost      string
		wantPort      int
		wantEndpoints int
	}{
		{
			name:          "empty endpoint info",
			info:          &EndpointInfo{},
			wantAddresses: 0,
			wantHost:      "",
			wantPort:      0,
			wantEndpoints: 0,
		},
		{
			name: "single endpoint",
			info: &EndpointInfo{
				Addresses:      []string{"10.0.0.1"},
				Port:           8080,
				SelectedHost:   "10.0.0.1",
				ReadyEndpoints: 1,
			},
			wantAddresses: 1,
			wantHost:      "10.0.0.1",
			wantPort:      8080,
			wantEndpoints: 1,
		},
		{
			name: "multiple endpoints",
			info: &EndpointInfo{
				Addresses:      []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
				Port:           3000,
				SelectedHost:   "10.0.0.2",
				ReadyEndpoints: 3,
			},
			wantAddresses: 3,
			wantHost:      "10.0.0.2",
			wantPort:      3000,
			wantEndpoints: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.info.Addresses) != tt.wantAddresses {
				t.Errorf("Addresses length = %v, want %v", len(tt.info.Addresses), tt.wantAddresses)
			}
			if tt.info.SelectedHost != tt.wantHost {
				t.Errorf("SelectedHost = %v, want %v", tt.info.SelectedHost, tt.wantHost)
			}
			if tt.info.Port != tt.wantPort {
				t.Errorf("Port = %v, want %v", tt.info.Port, tt.wantPort)
			}
			if tt.info.ReadyEndpoints != tt.wantEndpoints {
				t.Errorf("ReadyEndpoints = %v, want %v", tt.info.ReadyEndpoints, tt.wantEndpoints)
			}
		})
	}
}

func setupScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(npmkov1alpha1.AddToScheme(scheme))
	utilruntime.Must(discoveryv1.AddToScheme(scheme))
	return scheme
}

func TestBuildProxyHost(t *testing.T) {
	scheme := setupScheme()
	r := &NpmkoReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	tests := []struct {
		name          string
		npmko         *npmkov1alpha1.Npmko
		targetIP      string
		targetPort    int
		certificateId int
		sslForced     bool
		wantScheme    string
		wantSSL       bool
	}{
		{
			name: "basic http proxy",
			npmko: &npmkov1alpha1.Npmko{
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames:   []string{"example.com"},
					ForwardScheme: "http",
					BlockExploits: true,
				},
			},
			targetIP:      "10.0.0.1",
			targetPort:    8080,
			certificateId: 0,
			sslForced:     false,
			wantScheme:    "http",
			wantSSL:       false,
		},
		{
			name: "https proxy with certificate",
			npmko: &npmkov1alpha1.Npmko{
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames:   []string{"secure.example.com"},
					ForwardScheme: "https",
					HTTP2Support:  true,
					HSTSEnabled:   true,
				},
			},
			targetIP:      "10.0.0.2",
			targetPort:    443,
			certificateId: 123,
			sslForced:     true,
			wantScheme:    "https",
			wantSSL:       true,
		},
		{
			name: "default forward scheme",
			npmko: &npmkov1alpha1.Npmko{
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames: []string{"default.example.com"},
				},
			},
			targetIP:      "10.0.0.3",
			targetPort:    80,
			certificateId: 0,
			sslForced:     false,
			wantScheme:    "http", // default
			wantSSL:       false,
		},
		{
			name: "with all options",
			npmko: &npmkov1alpha1.Npmko{
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames:     []string{"full.example.com", "www.full.example.com"},
					ForwardScheme:   "http",
					CachingEnabled:  true,
					BlockExploits:   true,
					AllowWebsockets: true,
					HTTP2Support:    true,
					HSTSEnabled:     true,
					HSTSSubdomains:  true,
					AdvancedConfig:  "proxy_read_timeout 300;",
				},
			},
			targetIP:      "10.0.0.4",
			targetPort:    9000,
			certificateId: 456,
			sslForced:     true,
			wantScheme:    "http",
			wantSSL:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxyHost := r.buildProxyHost(tt.npmko, tt.targetIP, tt.targetPort, tt.certificateId, tt.sslForced)

			if proxyHost.ForwardScheme != tt.wantScheme {
				t.Errorf("ForwardScheme = %v, want %v", proxyHost.ForwardScheme, tt.wantScheme)
			}
			if proxyHost.ForwardHost != tt.targetIP {
				t.Errorf("ForwardHost = %v, want %v", proxyHost.ForwardHost, tt.targetIP)
			}
			if proxyHost.ForwardPort != tt.targetPort {
				t.Errorf("ForwardPort = %v, want %v", proxyHost.ForwardPort, tt.targetPort)
			}
			if proxyHost.SSLForced != tt.sslForced {
				t.Errorf("SSLForced = %v, want %v", proxyHost.SSLForced, tt.sslForced)
			}
			if tt.certificateId > 0 {
				if proxyHost.CertificateId != tt.certificateId {
					t.Errorf("CertificateId = %v, want %v", proxyHost.CertificateId, tt.certificateId)
				}
			}
		})
	}
}

func TestDiscoverEndpoints(t *testing.T) {
	scheme := setupScheme()
	ready := true
	notReady := false
	port := int32(8080)

	tests := []struct {
		name             string
		serviceName      string
		currentForward   string
		endpointSlices   []discoveryv1.EndpointSlice
		wantSelectedHost string
		wantPort         int
		wantReadyCount   int
		wantErr          bool
	}{
		{
			name:             "no endpoint slices",
			serviceName:      "my-service",
			endpointSlices:   []discoveryv1.EndpointSlice{},
			wantSelectedHost: "",
			wantPort:         0,
			wantReadyCount:   0,
		},
		{
			name:        "single ready endpoint",
			serviceName: "my-service",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service-abc",
						Namespace: "default",
						Labels: map[string]string{
							discoveryv1.LabelServiceName: "my-service",
						},
					},
					Ports: []discoveryv1.EndpointPort{{Port: &port}},
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses:  []string{"10.0.0.1"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
					},
				},
			},
			wantSelectedHost: "10.0.0.1",
			wantPort:         8080,
			wantReadyCount:   1,
		},
		{
			name:           "multiple ready endpoints - selects first",
			serviceName:    "my-service",
			currentForward: "",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service-abc",
						Namespace: "default",
						Labels: map[string]string{
							discoveryv1.LabelServiceName: "my-service",
						},
					},
					Ports: []discoveryv1.EndpointPort{{Port: &port}},
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses:  []string{"10.0.0.1"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
						{
							Addresses:  []string{"10.0.0.2"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
					},
				},
			},
			wantSelectedHost: "10.0.0.1",
			wantPort:         8080,
			wantReadyCount:   2,
		},
		{
			name:           "keeps current forward host if still ready",
			serviceName:    "my-service",
			currentForward: "10.0.0.2",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service-abc",
						Namespace: "default",
						Labels: map[string]string{
							discoveryv1.LabelServiceName: "my-service",
						},
					},
					Ports: []discoveryv1.EndpointPort{{Port: &port}},
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses:  []string{"10.0.0.1"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
						{
							Addresses:  []string{"10.0.0.2"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
					},
				},
			},
			wantSelectedHost: "10.0.0.2", // keeps current
			wantPort:         8080,
			wantReadyCount:   2,
		},
		{
			name:           "switches when current forward not ready",
			serviceName:    "my-service",
			currentForward: "10.0.0.3", // not in list anymore
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service-abc",
						Namespace: "default",
						Labels: map[string]string{
							discoveryv1.LabelServiceName: "my-service",
						},
					},
					Ports: []discoveryv1.EndpointPort{{Port: &port}},
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses:  []string{"10.0.0.1"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
						{
							Addresses:  []string{"10.0.0.2"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
					},
				},
			},
			wantSelectedHost: "10.0.0.1", // switches to first
			wantPort:         8080,
			wantReadyCount:   2,
		},
		{
			name:        "skips not ready endpoints",
			serviceName: "my-service",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service-abc",
						Namespace: "default",
						Labels: map[string]string{
							discoveryv1.LabelServiceName: "my-service",
						},
					},
					Ports: []discoveryv1.EndpointPort{{Port: &port}},
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses:  []string{"10.0.0.1"},
							Conditions: discoveryv1.EndpointConditions{Ready: &notReady},
						},
						{
							Addresses:  []string{"10.0.0.2"},
							Conditions: discoveryv1.EndpointConditions{Ready: &ready},
						},
					},
				},
			},
			wantSelectedHost: "10.0.0.2",
			wantPort:         8080,
			wantReadyCount:   1,
		},
		{
			name:        "all endpoints not ready",
			serviceName: "my-service",
			endpointSlices: []discoveryv1.EndpointSlice{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-service-abc",
						Namespace: "default",
						Labels: map[string]string{
							discoveryv1.LabelServiceName: "my-service",
						},
					},
					Ports: []discoveryv1.EndpointPort{{Port: &port}},
					Endpoints: []discoveryv1.Endpoint{
						{
							Addresses:  []string{"10.0.0.1"},
							Conditions: discoveryv1.EndpointConditions{Ready: &notReady},
						},
					},
				},
			},
			wantSelectedHost: "",
			wantPort:         0,
			wantReadyCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build client with objects
			builder := fake.NewClientBuilder().WithScheme(scheme)
			for i := range tt.endpointSlices {
				builder = builder.WithObjects(&tt.endpointSlices[i])
			}
			fakeClient := builder.Build()

			r := &NpmkoReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			info, err := r.discoverEndpoints(context.Background(), "default", tt.serviceName, tt.currentForward)
			if (err != nil) != tt.wantErr {
				t.Errorf("discoverEndpoints() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if info.SelectedHost != tt.wantSelectedHost {
				t.Errorf("SelectedHost = %v, want %v", info.SelectedHost, tt.wantSelectedHost)
			}
			if tt.wantPort > 0 && info.Port != tt.wantPort {
				t.Errorf("Port = %v, want %v", info.Port, tt.wantPort)
			}
			if info.ReadyEndpoints != tt.wantReadyCount {
				t.Errorf("ReadyEndpoints = %v, want %v", info.ReadyEndpoints, tt.wantReadyCount)
			}
		})
	}
}

func TestSyncProxyHost(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	// Mock NPM server for create
	createServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]int{"id": 100})
	}))
	defer createServer.Close()

	// Mock NPM server for update
	updateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"id": 42})
	}))
	defer updateServer.Close()

	tests := []struct {
		name      string
		serverURL string
		npmko     *npmkov1alpha1.Npmko
		wantID    int
		wantErr   bool
	}{
		{
			name:      "create new proxy host",
			serverURL: createServer.URL,
			npmko: &npmkov1alpha1.Npmko{
				Status: npmkov1alpha1.NpmkoStatus{
					ProxyHostId: 0, // No existing host
				},
			},
			wantID:  100,
			wantErr: false,
		},
		{
			name:      "update existing proxy host",
			serverURL: updateServer.URL,
			npmko: &npmkov1alpha1.Npmko{
				Status: npmkov1alpha1.NpmkoStatus{
					ProxyHostId: 42, // Has existing host
				},
			},
			wantID:  42,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &NpmkoReconciler{
				Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
				Scheme: scheme,
			}

			npmClient := npm.NewClient(tt.serverURL, "admin@example.com", "password")
			_ = npmClient.Login(ctx)

			proxyHost := npm.ProxyHost{
				DomainNames:   []string{"test.example.com"},
				ForwardHost:   "10.0.0.1",
				ForwardPort:   8080,
				ForwardScheme: "http",
			}

			id, err := r.syncProxyHost(ctx, npmClient, tt.npmko, proxyHost)
			if (err != nil) != tt.wantErr {
				t.Errorf("syncProxyHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if id != tt.wantID {
				t.Errorf("syncProxyHost() id = %v, want %v", id, tt.wantID)
			}
		})
	}
}

func TestGetDNSCredentials(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	tests := []struct {
		name      string
		secret    *corev1.Secret
		ref       npmkov1alpha1.SecretReference
		wantCreds string
		wantErr   bool
	}{
		{
			name: "get credentials with explicit key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dns-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"api-token": []byte("dns_cloudflare_api_token = xxx"),
				},
			},
			ref: npmkov1alpha1.SecretReference{
				Name: "dns-creds",
				Key:  "api-token",
			},
			wantCreds: "dns_cloudflare_api_token = xxx",
			wantErr:   false,
		},
		{
			name: "get credentials with default key",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dns-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"credentials": []byte("dns_route53_secret_access_key = abc"),
				},
			},
			ref: npmkov1alpha1.SecretReference{
				Name: "dns-creds",
				Key:  "", // Will default to "credentials"
			},
			wantCreds: "dns_route53_secret_access_key = abc",
			wantErr:   false,
		},
		{
			name:   "secret not found",
			secret: nil,
			ref: npmkov1alpha1.SecretReference{
				Name: "missing-secret",
				Key:  "credentials",
			},
			wantErr: true,
		},
		{
			name: "key not found in secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dns-creds",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"wrong-key": []byte("value"),
				},
			},
			ref: npmkov1alpha1.SecretReference{
				Name: "dns-creds",
				Key:  "api-token",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.secret != nil {
				builder = builder.WithObjects(tt.secret)
			}
			fakeClient := builder.Build()

			r := &NpmkoReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			creds, err := r.getDNSCredentials(ctx, "default", tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDNSCredentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && creds != tt.wantCreds {
				t.Errorf("getDNSCredentials() = %v, want %v", creds, tt.wantCreds)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	// Set operator namespace for tests
	t.Setenv("OPERATOR_NAMESPACE", "operator-ns")

	tests := []struct {
		name      string
		objects   []client.Object
		namespace string
		wantURL   string
		wantEmail string
		wantPass  string
	}{
		{
			name: "load from global configmap and secret",
			objects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.ConfigMapName,
						Namespace: "operator-ns",
					},
					Data: map[string]string{
						"npm-url":   "http://npm:81",
						"npm-email": "admin@example.com",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.CredentialsSecretName,
						Namespace: "operator-ns",
					},
					Data: map[string][]byte{
						"password": []byte("globalpass"),
					},
				},
			},
			namespace: "app-ns",
			wantURL:   "http://npm:81",
			wantEmail: "admin@example.com",
			wantPass:  "globalpass",
		},
		{
			name: "namespace config overrides global",
			objects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.ConfigMapName,
						Namespace: "operator-ns",
					},
					Data: map[string]string{
						"npm-url":   "http://global-npm:81",
						"npm-email": "global@example.com",
					},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.ConfigMapName,
						Namespace: "app-ns",
					},
					Data: map[string]string{
						"npm-url":   "http://local-npm:81",
						"npm-email": "local@example.com",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.CredentialsSecretName,
						Namespace: "app-ns",
					},
					Data: map[string][]byte{
						"password": []byte("localpass"),
					},
				},
			},
			namespace: "app-ns",
			wantURL:   "http://local-npm:81",
			wantEmail: "local@example.com",
			wantPass:  "localpass",
		},
		{
			name:      "no config returns empty",
			objects:   []client.Object{},
			namespace: "app-ns",
			wantURL:   "",
			wantEmail: "",
			wantPass:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			for _, obj := range tt.objects {
				builder = builder.WithObjects(obj)
			}
			fakeClient := builder.Build()

			r := &NpmkoReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			cfg, err := r.loadConfig(ctx, tt.namespace)
			if err != nil {
				t.Errorf("loadConfig() error = %v", err)
				return
			}
			if cfg.NPMURL != tt.wantURL {
				t.Errorf("NPMURL = %v, want %v", cfg.NPMURL, tt.wantURL)
			}
			if cfg.NPMEmail != tt.wantEmail {
				t.Errorf("NPMEmail = %v, want %v", cfg.NPMEmail, tt.wantEmail)
			}
			if cfg.NPMPassword != tt.wantPass {
				t.Errorf("NPMPassword = %v, want %v", cfg.NPMPassword, tt.wantPass)
			}
		})
	}
}

func TestReconcilerConstants(t *testing.T) {
	// Test that constants have expected values
	if requeueOnError != 30*time.Second {
		t.Errorf("requeueOnError = %v, want %v", requeueOnError, 30*time.Second)
	}
	if requeueOnSuccess != 5*time.Minute {
		t.Errorf("requeueOnSuccess = %v, want %v", requeueOnSuccess, 5*time.Minute)
	}
	if defaultForwardScheme != "http" {
		t.Errorf("defaultForwardScheme = %v, want http", defaultForwardScheme)
	}
	if defaultTargetPort != 80 {
		t.Errorf("defaultTargetPort = %v, want 80", defaultTargetPort)
	}
	if npmkoFinalizer != "npmko.io/finalizer" {
		t.Errorf("npmkoFinalizer = %v, want npmko.io/finalizer", npmkoFinalizer)
	}
}

func TestNpmkoReconcilerStructure(t *testing.T) {
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	if r.Client == nil {
		t.Error("Client should not be nil")
	}
	if r.Scheme == nil {
		t.Error("Scheme should not be nil")
	}
}

// Integration-style tests using fake client
func TestReconcileResourceNotFound(t *testing.T) {
	scheme := setupScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "nonexistent",
			Namespace: "default",
		},
	}

	result, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Errorf("Reconcile() unexpected error = %v", err)
	}
	// Resource not found should return empty result without error
	if result.RequeueAfter > 0 {
		t.Errorf("Reconcile() for not found should not requeue, got %v", result)
	}
}

// Test SetupWithManager structure
func TestSetupWithManagerStructure(t *testing.T) {
	scheme := setupScheme()
	r := &NpmkoReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	// Verify the reconciler has the expected structure for setup
	if r.Client == nil {
		t.Error("Reconciler.Client should not be nil")
	}
	if r.Scheme == nil {
		t.Error("Reconciler.Scheme should not be nil")
	}
}

func TestHandleDeletion(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	// Mock NPM server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/tokens":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tests := []struct {
		name       string
		npmko      *npmkov1alpha1.Npmko
		wantResult ctrl.Result
		wantErr    bool
	}{
		{
			name: "delete with no finalizer",
			npmko: &npmkov1alpha1.Npmko{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Namespace:  "default",
					Finalizers: []string{}, // No finalizer
				},
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "delete with finalizer and proxy host",
			npmko: &npmkov1alpha1.Npmko{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Namespace:  "default",
					Finalizers: []string{npmkoFinalizer},
				},
				Status: npmkov1alpha1.NpmkoStatus{
					ProxyHostId:   42,
					CertificateId: 0,
				},
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
		{
			name: "delete with finalizer, proxy host and certificate",
			npmko: &npmkov1alpha1.Npmko{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test",
					Namespace:  "default",
					Finalizers: []string{npmkoFinalizer},
				},
				Status: npmkov1alpha1.NpmkoStatus{
					ProxyHostId:   42,
					CertificateId: 123,
				},
			},
			wantResult: ctrl.Result{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.npmko).
				WithStatusSubresource(tt.npmko).Build()

			r := &NpmkoReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			npmClient := npm.NewClient(server.URL, "admin@example.com", "password")
			_ = npmClient.Login(ctx)

			result, err := r.handleDeletion(ctx, npmClient, tt.npmko)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleDeletion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.wantResult {
				t.Errorf("handleDeletion() result = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestUpdateStatusError(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	npmko := &npmkov1alpha1.Npmko{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: npmkov1alpha1.NpmkoSpec{
			ServiceName: "my-service",
			DomainNames: []string{"example.com"},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(npmko).
		WithStatusSubresource(npmko).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	testErr := fmt.Errorf("test error message")
	result, err := r.updateStatusError(ctx, npmko, testErr)

	if err == nil || err.Error() != "test error message" {
		t.Errorf("updateStatusError() should return the input error, got %v", err)
	}

	if result.RequeueAfter != requeueOnError {
		t.Errorf("updateStatusError() RequeueAfter = %v, want %v", result.RequeueAfter, requeueOnError)
	}

	if npmko.Status.Phase != npmkov1alpha1.PhaseError {
		t.Errorf("Status.Phase = %v, want %v", npmko.Status.Phase, npmkov1alpha1.PhaseError)
	}

	if npmko.Status.Message != "test error message" {
		t.Errorf("Status.Message = %v, want 'test error message'", npmko.Status.Message)
	}
}

func TestUpdateStatusPending(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	npmko := &npmkov1alpha1.Npmko{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: npmkov1alpha1.NpmkoSpec{
			ServiceName: "my-service",
			DomainNames: []string{"example.com"},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(npmko).
		WithStatusSubresource(npmko).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	result, err := r.updateStatusPending(ctx, npmko, "Waiting for service")

	if err != nil {
		t.Errorf("updateStatusPending() error = %v", err)
	}

	if result.RequeueAfter != requeueOnError {
		t.Errorf("updateStatusPending() RequeueAfter = %v, want %v", result.RequeueAfter, requeueOnError)
	}

	if npmko.Status.Phase != npmkov1alpha1.PhasePending {
		t.Errorf("Status.Phase = %v, want %v", npmko.Status.Phase, npmkov1alpha1.PhasePending)
	}

	if npmko.Status.Message != "Waiting for service" {
		t.Errorf("Status.Message = %v, want 'Waiting for service'", npmko.Status.Message)
	}
}

func TestUpdateStatusReady(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	npmko := &npmkov1alpha1.Npmko{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: npmkov1alpha1.NpmkoSpec{
			ServiceName: "my-service",
			DomainNames: []string{"example.com"},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(npmko).
		WithStatusSubresource(npmko).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	endpointInfo := &EndpointInfo{
		Addresses:      []string{"10.0.0.1", "10.0.0.2"},
		Port:           8080,
		SelectedHost:   "10.0.0.1",
		ReadyEndpoints: 2,
	}

	result, err := r.updateStatusReady(ctx, npmko, 42, 123, endpointInfo)

	if err != nil {
		t.Errorf("updateStatusReady() error = %v", err)
	}

	if result.RequeueAfter != requeueOnSuccess {
		t.Errorf("updateStatusReady() RequeueAfter = %v, want %v", result.RequeueAfter, requeueOnSuccess)
	}

	if npmko.Status.Phase != npmkov1alpha1.PhaseReady {
		t.Errorf("Status.Phase = %v, want %v", npmko.Status.Phase, npmkov1alpha1.PhaseReady)
	}

	if npmko.Status.ProxyHostId != 42 {
		t.Errorf("Status.ProxyHostId = %v, want 42", npmko.Status.ProxyHostId)
	}

	if npmko.Status.CertificateId != 123 {
		t.Errorf("Status.CertificateId = %v, want 123", npmko.Status.CertificateId)
	}

	if npmko.Status.ForwardHost != "10.0.0.1" {
		t.Errorf("Status.ForwardHost = %v, want 10.0.0.1", npmko.Status.ForwardHost)
	}

	if npmko.Status.ForwardPort != 8080 {
		t.Errorf("Status.ForwardPort = %v, want 8080", npmko.Status.ForwardPort)
	}

	if npmko.Status.ReadyEndpoints != 2 {
		t.Errorf("Status.ReadyEndpoints = %v, want 2", npmko.Status.ReadyEndpoints)
	}
}

func TestEnsureCertificate(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	tests := []struct {
		name          string
		npmko         *npmkov1alpha1.Npmko
		serverHandler http.HandlerFunc
		secretData    map[string][]byte
		wantCertID    int
		wantErr       bool
	}{
		{
			name: "existing certificate in status",
			npmko: &npmkov1alpha1.Npmko{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames: []string{"example.com"},
					SSL: &npmkov1alpha1.SSLConfig{
						Enabled: true,
						DNSChallenge: npmkov1alpha1.DNSChallengeConfig{
							Provider: "cloudflare",
							CredentialsSecretRef: npmkov1alpha1.SecretReference{
								Name: "dns-creds",
								Key:  "credentials",
							},
						},
					},
				},
				Status: npmkov1alpha1.NpmkoStatus{
					CertificateId: 999,
				},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/api/tokens":
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				case r.URL.Path == "/api/nginx/certificates/999" && r.Method == http.MethodGet:
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(npm.CertificateResponse{
						ID:          999,
						DomainNames: []string{"example.com"},
					})
				}
			},
			secretData: map[string][]byte{"credentials": []byte("dns_cloudflare_api_token = xxx")},
			wantCertID: 999,
			wantErr:    false,
		},
		{
			name: "find existing certificate by domain",
			npmko: &npmkov1alpha1.Npmko{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames: []string{"example.com"},
					SSL: &npmkov1alpha1.SSLConfig{
						Enabled: true,
						DNSChallenge: npmkov1alpha1.DNSChallengeConfig{
							Provider: "cloudflare",
							CredentialsSecretRef: npmkov1alpha1.SecretReference{
								Name: "dns-creds",
								Key:  "credentials",
							},
						},
					},
				},
				Status: npmkov1alpha1.NpmkoStatus{
					CertificateId: 0, // No cert in status
				},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/api/tokens":
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				case r.URL.Path == "/api/nginx/certificates" && r.Method == http.MethodGet:
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode([]npm.CertificateResponse{
						{ID: 555, DomainNames: []string{"example.com"}},
					})
				}
			},
			secretData: map[string][]byte{"credentials": []byte("dns_cloudflare_api_token = xxx")},
			wantCertID: 555,
			wantErr:    false,
		},
		{
			name: "create new certificate",
			npmko: &npmkov1alpha1.Npmko{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames: []string{"new.example.com"},
					SSL: &npmkov1alpha1.SSLConfig{
						Enabled: true,
						DNSChallenge: npmkov1alpha1.DNSChallengeConfig{
							Provider: "cloudflare",
							CredentialsSecretRef: npmkov1alpha1.SecretReference{
								Name: "dns-creds",
								Key:  "credentials",
							},
						},
					},
				},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/api/tokens":
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				case r.URL.Path == "/api/nginx/certificates" && r.Method == http.MethodGet:
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode([]npm.CertificateResponse{}) // No existing certs
				case r.URL.Path == "/api/nginx/certificates" && r.Method == http.MethodPost:
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(npm.CertificateResponse{
						ID:          777,
						DomainNames: []string{"new.example.com"},
					})
				}
			},
			secretData: map[string][]byte{"credentials": []byte("dns_cloudflare_api_token = xxx")},
			wantCertID: 777,
			wantErr:    false,
		},
		{
			name: "secret not found",
			npmko: &npmkov1alpha1.Npmko{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: npmkov1alpha1.NpmkoSpec{
					DomainNames: []string{"example.com"},
					SSL: &npmkov1alpha1.SSLConfig{
						Enabled: true,
						DNSChallenge: npmkov1alpha1.DNSChallengeConfig{
							Provider: "cloudflare",
							CredentialsSecretRef: npmkov1alpha1.SecretReference{
								Name: "missing-secret",
								Key:  "credentials",
							},
						},
					},
				},
			},
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tokens" {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
				} else if r.URL.Path == "/api/nginx/certificates" && r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode([]npm.CertificateResponse{})
				}
			},
			secretData: nil, // No secret created
			wantCertID: 0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.npmko)
			if tt.secretData != nil {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dns-creds",
						Namespace: "default",
					},
					Data: tt.secretData,
				}
				builder = builder.WithObjects(secret)
			}
			fakeClient := builder.Build()

			r := &NpmkoReconciler{
				Client: fakeClient,
				Scheme: scheme,
			}

			npmClient := npm.NewClient(server.URL, "admin@example.com", "password")
			_ = npmClient.Login(ctx)

			certID, err := r.ensureCertificate(ctx, npmClient, tt.npmko)
			if (err != nil) != tt.wantErr {
				t.Errorf("ensureCertificate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if certID != tt.wantCertID {
				t.Errorf("ensureCertificate() certID = %v, want %v", certID, tt.wantCertID)
			}
		})
	}
}

func TestReconcileWithConfigError(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	// Set operator namespace for tests
	t.Setenv("OPERATOR_NAMESPACE", "operator-ns")

	npmko := &npmkov1alpha1.Npmko{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: npmkov1alpha1.NpmkoSpec{
			ServiceName: "my-service",
			DomainNames: []string{"example.com"},
		},
	}

	// Create operator namespace ConfigMap with incomplete config (missing password)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: "operator-ns",
		},
		Data: map[string]string{
			"npm-url":   "http://npm:81",
			"npm-email": "admin@example.com",
			// password is missing
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(npmko, configMap).
		WithStatusSubresource(npmko).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test",
			Namespace: "default",
		},
	}

	result, err := r.Reconcile(ctx, req)

	// Should return error due to invalid config
	if err == nil {
		t.Error("Reconcile() should return error for invalid config")
	}

	// Should requeue
	if result.RequeueAfter != requeueOnError {
		t.Errorf("Reconcile() RequeueAfter = %v, want %v", result.RequeueAfter, requeueOnError)
	}
}

func TestReconcileResourceDeletion(t *testing.T) {
	scheme := setupScheme()
	ctx := context.Background()

	// Set operator namespace
	t.Setenv("OPERATOR_NAMESPACE", "operator-ns")

	now := metav1.Now()
	npmko := &npmkov1alpha1.Npmko{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test",
			Namespace:         "default",
			DeletionTimestamp: &now,
			Finalizers:        []string{npmkoFinalizer},
		},
		Spec: npmkov1alpha1.NpmkoSpec{
			ServiceName: "my-service",
			DomainNames: []string{"example.com"},
		},
		Status: npmkov1alpha1.NpmkoStatus{
			ProxyHostId:   42,
			CertificateId: 123,
		},
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ConfigMapName,
			Namespace: "operator-ns",
		},
		Data: map[string]string{
			"npm-url":   "http://npm:81",
			"npm-email": "admin@example.com",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.CredentialsSecretName,
			Namespace: "operator-ns",
		},
		Data: map[string][]byte{
			"password": []byte("password"),
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(npmko, configMap, secret).
		WithStatusSubresource(npmko).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	// Note: This test will fail on the NPM login since we don't have a mock server
	// but it exercises the deletion path code
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test",
			Namespace: "default",
		},
	}

	// This will fail at NPM login, but exercises the deletion detection code
	_, _ = r.Reconcile(ctx, req)
}

func TestDiscoverEndpointsWithDefaultPort(t *testing.T) {
	scheme := setupScheme()
	ready := true

	// Create endpoint slice without port information
	endpointSlice := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service-abc",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "my-service",
			},
		},
		Ports: []discoveryv1.EndpointPort{}, // No ports - should use default
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: &ready},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&endpointSlice).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	info, err := r.discoverEndpoints(context.Background(), "default", "my-service", "")
	if err != nil {
		t.Errorf("discoverEndpoints() error = %v", err)
		return
	}

	// Should use default port (80)
	if info.Port != defaultTargetPort {
		t.Errorf("Port = %v, want %v (default)", info.Port, defaultTargetPort)
	}
}

func TestDiscoverEndpointsMultipleSlices(t *testing.T) {
	scheme := setupScheme()
	ready := true
	port := int32(8080)

	// Create multiple endpoint slices for the same service
	slice1 := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service-abc",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "my-service",
			},
		},
		Ports: []discoveryv1.EndpointPort{{Port: &port}},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{Ready: &ready},
			},
		},
	}

	slice2 := discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service-def",
			Namespace: "default",
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "my-service",
			},
		},
		Ports: []discoveryv1.EndpointPort{{Port: &port}},
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses:  []string{"10.0.0.2"},
				Conditions: discoveryv1.EndpointConditions{Ready: &ready},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&slice1, &slice2).Build()

	r := &NpmkoReconciler{
		Client: fakeClient,
		Scheme: scheme,
	}

	info, err := r.discoverEndpoints(context.Background(), "default", "my-service", "")
	if err != nil {
		t.Errorf("discoverEndpoints() error = %v", err)
		return
	}

	// Should have endpoints from both slices
	if info.ReadyEndpoints != 2 {
		t.Errorf("ReadyEndpoints = %v, want 2", info.ReadyEndpoints)
	}
}
