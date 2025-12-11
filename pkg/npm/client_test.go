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
