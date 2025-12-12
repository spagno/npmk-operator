package npm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("http://npm:81", "admin@example.com", "password")

	if client.baseURL != "http://npm:81" {
		t.Errorf("NewClient().baseURL = %v, want %v", client.baseURL, "http://npm:81")
	}
	if client.email != "admin@example.com" {
		t.Errorf("NewClient().email = %v, want %v", client.email, "admin@example.com")
	}
	if client.password != "password" {
		t.Errorf("NewClient().password = %v, want %v", client.password, "password")
	}
	if client.httpClient == nil {
		t.Error("NewClient().httpClient should not be nil")
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErr    bool
	}{
		{
			name:       "successful login",
			statusCode: http.StatusOK,
			response:   map[string]string{"token": "test-token-123"},
			wantErr:    false,
		},
		{
			name:       "invalid credentials",
			statusCode: http.StatusUnauthorized,
			response:   map[string]string{"error": "Invalid credentials"},
			wantErr:    true,
		},
		{
			name:       "empty token response",
			statusCode: http.StatusOK,
			response:   map[string]string{"token": ""},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/tokens" {
					t.Errorf("Expected path /api/tokens, got %s", r.URL.Path)
				}
				if r.Method != http.MethodPost {
					t.Errorf("Expected POST method, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin@example.com", "password")
			err := client.Login(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("Login() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && client.token != "test-token-123" {
				t.Errorf("Login() token = %v, want %v", client.token, "test-token-123")
			}
		})
	}
}

func TestCreateProxyHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/nginx/proxy-hosts" {
			t.Errorf("Expected path /api/nginx/proxy-hosts, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		// Verify authorization header
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]int{"id": 42})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	host := ProxyHost{
		DomainNames:   []string{"example.com"},
		ForwardHost:   "10.0.0.1",
		ForwardPort:   8080,
		ForwardScheme: "http",
	}

	id, err := client.CreateProxyHost(context.Background(), host)
	if err != nil {
		t.Errorf("CreateProxyHost() error = %v", err)
	}
	if id != 42 {
		t.Errorf("CreateProxyHost() id = %v, want %v", id, 42)
	}
}

func TestUpdateProxyHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/nginx/proxy-hosts/42" {
			t.Errorf("Expected path /api/nginx/proxy-hosts/42, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("Expected PUT method, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]int{"id": 42})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	host := ProxyHost{
		DomainNames:   []string{"example.com"},
		ForwardHost:   "10.0.0.2",
		ForwardPort:   8080,
		ForwardScheme: "http",
	}

	id, err := client.UpdateProxyHost(context.Background(), 42, host)
	if err != nil {
		t.Errorf("UpdateProxyHost() error = %v", err)
	}
	if id != 42 {
		t.Errorf("UpdateProxyHost() id = %v, want %v", id, 42)
	}
}

func TestDeleteProxyHost(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful delete with 200",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "successful delete with 204",
			statusCode: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("Expected DELETE method, got %s", r.Method)
				}
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin@example.com", "password")
			client.token = "test-token"

			err := client.DeleteProxyHost(context.Background(), 42)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteProxyHost() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateLetsEncryptCertificate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/nginx/certificates" {
			t.Errorf("Expected path /api/nginx/certificates, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST method, got %s", r.Method)
		}

		var req CertificateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Provider != "letsencrypt" {
			t.Errorf("Expected provider letsencrypt, got %s", req.Provider)
		}
		if !req.Meta.DNSChallenge {
			t.Error("Expected DNSChallenge to be true")
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CertificateResponse{
			ID:          123,
			DomainNames: req.DomainNames,
			Provider:    "letsencrypt",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	propagation := 30
	cert, err := client.CreateLetsEncryptCertificate(
		context.Background(),
		[]string{"example.com"},
		"cloudflare",
		"dns_cloudflare_api_token = xxx",
		&propagation,
	)

	if err != nil {
		t.Errorf("CreateLetsEncryptCertificate() error = %v", err)
	}
	if cert.ID != 123 {
		t.Errorf("CreateLetsEncryptCertificate() ID = %v, want %v", cert.ID, 123)
	}
}

func TestListCertificates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/nginx/certificates" {
			t.Errorf("Expected path /api/nginx/certificates, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET method, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]CertificateResponse{
			{ID: 1, DomainNames: []string{"a.com"}},
			{ID: 2, DomainNames: []string{"b.com"}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	certs, err := client.ListCertificates(context.Background())
	if err != nil {
		t.Errorf("ListCertificates() error = %v", err)
	}
	if len(certs) != 2 {
		t.Errorf("ListCertificates() count = %v, want %v", len(certs), 2)
	}
}

func TestFindCertificateByDomains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]CertificateResponse{
			{ID: 1, DomainNames: []string{"a.com", "www.a.com"}},
			{ID: 2, DomainNames: []string{"b.com"}},
			{ID: 3, DomainNames: []string{"c.com", "d.com"}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	tests := []struct {
		name    string
		domains []string
		wantID  int
		wantNil bool
	}{
		{
			name:    "find exact match",
			domains: []string{"b.com"},
			wantID:  2,
		},
		{
			name:    "find multi-domain match",
			domains: []string{"a.com", "www.a.com"},
			wantID:  1,
		},
		{
			name:    "no match - different domains",
			domains: []string{"x.com"},
			wantNil: true,
		},
		{
			name:    "no match - partial domains",
			domains: []string{"a.com"},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := client.FindCertificateByDomains(context.Background(), tt.domains)
			if err != nil {
				t.Errorf("FindCertificateByDomains() error = %v", err)
			}
			if tt.wantNil {
				if cert != nil {
					t.Errorf("FindCertificateByDomains() = %v, want nil", cert)
				}
			} else {
				if cert == nil {
					t.Error("FindCertificateByDomains() = nil, want non-nil")
				} else if cert.ID != tt.wantID {
					t.Errorf("FindCertificateByDomains() ID = %v, want %v", cert.ID, tt.wantID)
				}
			}
		})
	}
}

func TestDomainsMatch(t *testing.T) {
	tests := []struct {
		name        string
		certDomains []string
		requested   map[string]bool
		want        bool
	}{
		{
			name:        "exact match single",
			certDomains: []string{"example.com"},
			requested:   map[string]bool{"example.com": true},
			want:        true,
		},
		{
			name:        "exact match multiple",
			certDomains: []string{"a.com", "b.com"},
			requested:   map[string]bool{"a.com": true, "b.com": true},
			want:        true,
		},
		{
			name:        "different length",
			certDomains: []string{"a.com", "b.com"},
			requested:   map[string]bool{"a.com": true},
			want:        false,
		},
		{
			name:        "different domains",
			certDomains: []string{"a.com"},
			requested:   map[string]bool{"b.com": true},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainsMatch(tt.certDomains, tt.requested)
			if got != tt.want {
				t.Errorf("domainsMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCertificate(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantNil    bool
		wantErr    bool
	}{
		{
			name:       "successful get",
			statusCode: http.StatusOK,
			response:   CertificateResponse{ID: 123, DomainNames: []string{"example.com"}},
			wantNil:    false,
			wantErr:    false,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			response:   nil,
			wantNil:    true,
			wantErr:    false,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			response:   map[string]string{"error": "internal error"},
			wantNil:    false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/nginx/certificates/123" {
					t.Errorf("Expected path /api/nginx/certificates/123, got %s", r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Errorf("Expected GET method, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
				if tt.response != nil {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin@example.com", "password")
			client.token = "test-token"

			cert, err := client.GetCertificate(context.Background(), 123)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCertificate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantNil && cert != nil {
				t.Errorf("GetCertificate() = %v, want nil", cert)
			}
			if !tt.wantNil && !tt.wantErr && cert == nil {
				t.Error("GetCertificate() = nil, want non-nil")
			}
			if !tt.wantNil && !tt.wantErr && cert != nil && cert.ID != 123 {
				t.Errorf("GetCertificate() ID = %v, want 123", cert.ID)
			}
		})
	}
}

func TestDeleteCertificate(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful delete with 200",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "successful delete with 204",
			statusCode: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "not found error",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/nginx/certificates/123" {
					t.Errorf("Expected path /api/nginx/certificates/123, got %s", r.URL.Path)
				}
				if r.Method != http.MethodDelete {
					t.Errorf("Expected DELETE method, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin@example.com", "password")
			client.token = "test-token"

			err := client.DeleteCertificate(context.Background(), 123)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteCertificate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateProxyHostError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid configuration"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	host := ProxyHost{
		DomainNames:   []string{"example.com"},
		ForwardHost:   "10.0.0.1",
		ForwardPort:   8080,
		ForwardScheme: "http",
	}

	_, err := client.CreateProxyHost(context.Background(), host)
	if err == nil {
		t.Error("CreateProxyHost() should return error for bad request")
	}
}

func TestUpdateProxyHostError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	host := ProxyHost{
		DomainNames:   []string{"example.com"},
		ForwardHost:   "10.0.0.2",
		ForwardPort:   8080,
		ForwardScheme: "http",
	}

	_, err := client.UpdateProxyHost(context.Background(), 999, host)
	if err == nil {
		t.Error("UpdateProxyHost() should return error for not found")
	}
}

func TestCreateLetsEncryptCertificateError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "DNS validation failed"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	propagation := 30
	_, err := client.CreateLetsEncryptCertificate(
		context.Background(),
		[]string{"example.com"},
		"cloudflare",
		"invalid_credentials",
		&propagation,
	)

	if err == nil {
		t.Error("CreateLetsEncryptCertificate() should return error for bad request")
	}
}

func TestListCertificatesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.ListCertificates(context.Background())
	if err == nil {
		t.Error("ListCertificates() should return error for server error")
	}
}

func TestDoRequestNetworkError(t *testing.T) {
	// Create client pointing to non-existent server
	client := NewClient("http://localhost:99999", "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.CreateProxyHost(context.Background(), ProxyHost{
		DomainNames: []string{"example.com"},
		ForwardHost: "10.0.0.1",
		ForwardPort: 8080,
	})

	if err == nil {
		t.Error("doRequest() should return error for network failure")
	}
}

func TestLoginNetworkError(t *testing.T) {
	// Create client pointing to non-existent server
	client := NewClient("http://localhost:99999", "admin@example.com", "password")

	err := client.Login(context.Background())
	if err == nil {
		t.Error("Login() should return error for network failure")
	}
}

func TestLoginDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return invalid JSON
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	err := client.Login(context.Background())
	if err == nil {
		t.Error("Login() should return error for invalid JSON response")
	}
}

func TestCreateProxyHostDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		// Return invalid JSON
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.CreateProxyHost(context.Background(), ProxyHost{
		DomainNames:   []string{"example.com"},
		ForwardHost:   "10.0.0.1",
		ForwardPort:   8080,
		ForwardScheme: "http",
	})
	if err == nil {
		t.Error("CreateProxyHost() should return error for invalid JSON response")
	}
}

func TestGetCertificateDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return invalid JSON
		_, _ = w.Write([]byte("invalid"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.GetCertificate(context.Background(), 123)
	if err == nil {
		t.Error("GetCertificate() should return error for invalid JSON response")
	}
}

func TestCreateLetsEncryptCertificateDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		// Return invalid JSON
		_, _ = w.Write([]byte("bad json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	propagation := 30
	_, err := client.CreateLetsEncryptCertificate(
		context.Background(),
		[]string{"example.com"},
		"cloudflare",
		"api_key=xxx",
		&propagation,
	)
	if err == nil {
		t.Error("CreateLetsEncryptCertificate() should return error for invalid JSON response")
	}
}

func TestListCertificatesDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Return invalid JSON
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.ListCertificates(context.Background())
	if err == nil {
		t.Error("ListCertificates() should return error for invalid JSON response")
	}
}

func TestFindCertificateByDomainsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.FindCertificateByDomains(context.Background(), []string{"example.com"})
	if err == nil {
		t.Error("FindCertificateByDomains() should return error when ListCertificates fails")
	}
}

func TestCreateLetsEncryptCertificateWithNilPropagation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CertificateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		if req.Meta.PropagationSeconds != nil {
			t.Error("Expected nil PropagationSeconds")
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(CertificateResponse{
			ID:          456,
			DomainNames: req.DomainNames,
			Provider:    "letsencrypt",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	// Test with nil propagation seconds
	cert, err := client.CreateLetsEncryptCertificate(
		context.Background(),
		[]string{"example.com"},
		"route53",
		"credentials_content",
		nil,
	)

	if err != nil {
		t.Errorf("CreateLetsEncryptCertificate() error = %v", err)
	}
	if cert.ID != 456 {
		t.Errorf("CreateLetsEncryptCertificate() ID = %v, want 456", cert.ID)
	}
}

func TestProxyHostFields(t *testing.T) {
	// Test that ProxyHost struct has all expected fields
	host := ProxyHost{
		DomainNames:           []string{"a.com", "b.com"},
		ForwardHost:           "10.0.0.1",
		ForwardPort:           8080,
		ForwardScheme:         "https",
		CachingEnabled:        true,
		BlockExploits:         true,
		AllowWebsocketUpgrade: true,
		CertificateId:         123,
		HTTP2Support:          true,
		HSTSEnabled:           true,
		HSTSSubdomains:        true,
		SSLForced:             true,
		AdvancedConfig:        "proxy_timeout 300;",
		AccessListId:          1,
		Meta:                  map[string]interface{}{"key": "value"},
		Locations:             []interface{}{"location1"},
	}

	if len(host.DomainNames) != 2 {
		t.Errorf("DomainNames length = %v, want 2", len(host.DomainNames))
	}
	if host.ForwardScheme != "https" {
		t.Errorf("ForwardScheme = %v, want https", host.ForwardScheme)
	}
	if !host.SSLForced {
		t.Error("SSLForced should be true")
	}
	if host.AdvancedConfig != "proxy_timeout 300;" {
		t.Errorf("AdvancedConfig = %v, want proxy_timeout 300;", host.AdvancedConfig)
	}
}

func TestCertificateRequestFields(t *testing.T) {
	propagation := 60
	req := CertificateRequest{
		DomainNames: []string{"test.com"},
		Provider:    "letsencrypt",
		Meta: CertificateMeta{
			DNSChallenge:           true,
			DNSProvider:            "cloudflare",
			DNSProviderCredentials: "api_token=xxx",
			PropagationSeconds:     &propagation,
		},
	}

	if req.Provider != "letsencrypt" {
		t.Errorf("Provider = %v, want letsencrypt", req.Provider)
	}
	if !req.Meta.DNSChallenge {
		t.Error("DNSChallenge should be true")
	}
	if req.Meta.DNSProvider != "cloudflare" {
		t.Errorf("DNSProvider = %v, want cloudflare", req.Meta.DNSProvider)
	}
	if *req.Meta.PropagationSeconds != 60 {
		t.Errorf("PropagationSeconds = %v, want 60", *req.Meta.PropagationSeconds)
	}
}

func TestCertificateResponseFields(t *testing.T) {
	resp := CertificateResponse{
		ID:          100,
		DomainNames: []string{"example.com", "www.example.com"},
		ExpiresOn:   "2025-12-31T00:00:00Z",
		Provider:    "letsencrypt",
	}

	if resp.ID != 100 {
		t.Errorf("ID = %v, want 100", resp.ID)
	}
	if len(resp.DomainNames) != 2 {
		t.Errorf("DomainNames length = %v, want 2", len(resp.DomainNames))
	}
	if resp.ExpiresOn != "2025-12-31T00:00:00Z" {
		t.Errorf("ExpiresOn = %v, want 2025-12-31T00:00:00Z", resp.ExpiresOn)
	}
}

func TestReadErrorBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "json error body",
			body: `{"error": "something went wrong"}`,
			want: `{"error": "something went wrong"}`,
		},
		{
			name: "plain text body",
			body: "Internal Server Error",
			want: "Internal Server Error",
		},
		{
			name: "empty body",
			body: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin@example.com", "password")
			client.token = "test-token"

			// This will trigger error path and call readErrorBody
			_, err := client.CreateProxyHost(context.Background(), ProxyHost{
				DomainNames: []string{"test.com"},
			})

			if err == nil {
				t.Error("Expected error")
			}
			// Error message should contain the body content
			if tt.body != "" && !contains(err.Error(), tt.body) {
				t.Errorf("Error message should contain body, got %v", err.Error())
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && s != "" && substr != "" &&
			(s[0:len(substr)] == substr || contains(s[1:], substr))))
}

func TestClientWithToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify token is set in Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-custom-token" {
			t.Errorf("Authorization header = %v, want Bearer my-custom-token", auth)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]CertificateResponse{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "my-custom-token"

	_, err := client.ListCertificates(context.Background())
	if err != nil {
		t.Errorf("ListCertificates() error = %v", err)
	}
}

func TestDeleteProxyHostNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "admin@example.com", "password")
	client.token = "test-token"

	err := client.DeleteProxyHost(context.Background(), 42)
	if err == nil {
		t.Error("DeleteProxyHost() should return error for network failure")
	}
}

func TestDeleteCertificateNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "admin@example.com", "password")
	client.token = "test-token"

	err := client.DeleteCertificate(context.Background(), 42)
	if err == nil {
		t.Error("DeleteCertificate() should return error for network failure")
	}
}

func TestGetCertificateNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.GetCertificate(context.Background(), 42)
	if err == nil {
		t.Error("GetCertificate() should return error for network failure")
	}
}

func TestUpdateProxyHostNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.UpdateProxyHost(context.Background(), 42, ProxyHost{})
	if err == nil {
		t.Error("UpdateProxyHost() should return error for network failure")
	}
}

func TestCreateLetsEncryptCertificateNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.CreateLetsEncryptCertificate(context.Background(), []string{"test.com"}, "cloudflare", "creds", nil)
	if err == nil {
		t.Error("CreateLetsEncryptCertificate() should return error for network failure")
	}
}

func TestListCertificatesNetworkError(t *testing.T) {
	client := NewClient("http://localhost:99999", "admin@example.com", "password")
	client.token = "test-token"

	_, err := client.ListCertificates(context.Background())
	if err == nil {
		t.Error("ListCertificates() should return error for network failure")
	}
}

func TestDoRequestWithNoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]CertificateResponse{})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "password")
	client.token = "test-token"

	// ListCertificates makes a GET request with no body
	certs, err := client.ListCertificates(context.Background())
	if err != nil {
		t.Errorf("ListCertificates() error = %v", err)
	}
	if certs == nil {
		t.Error("Expected non-nil response")
	}
}

func TestDefaultTimeout(t *testing.T) {
	// Verify that defaultTimeout is properly set
	if defaultTimeout.Seconds() != 30 {
		t.Errorf("defaultTimeout = %v, want 30s", defaultTimeout)
	}
}
