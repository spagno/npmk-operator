package npm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
)

// Client handles communication with the NPM API
type Client struct {
	baseURL    string
	email      string
	password   string
	token      string
	httpClient *http.Client
}

// ProxyHost represents an NPM proxy host configuration
type ProxyHost struct {
	DomainNames           []string               `json:"domain_names"`
	ForwardHost           string                 `json:"forward_host"`
	ForwardPort           int                    `json:"forward_port"`
	ForwardScheme         string                 `json:"forward_scheme"`
	CachingEnabled        bool                   `json:"caching_enabled"`
	BlockExploits         bool                   `json:"block_exploits"`
	AllowWebsocketUpgrade bool                   `json:"allow_websocket_upgrade"`
	CertificateId         interface{}            `json:"certificate_id"` // can be int or string "new"
	HTTP2Support          bool                   `json:"http2_support"`
	HSTSEnabled           bool                   `json:"hsts_enabled"`
	HSTSSubdomains        bool                   `json:"hsts_subdomains"`
	SSLForced             bool                   `json:"ssl_forced"`
	AdvancedConfig        string                 `json:"advanced_config"`
	AccessListId          interface{}            `json:"access_list_id"` // can be int or string "0"
	Meta                  map[string]interface{} `json:"meta"`
	Locations             []interface{}          `json:"locations"`
}

// CertificateRequest represents a Let's Encrypt certificate request
type CertificateRequest struct {
	DomainNames []string        `json:"domain_names"`
	Provider    string          `json:"provider"`
	Meta        CertificateMeta `json:"meta"`
}

// CertificateMeta contains Let's Encrypt DNS challenge configuration
type CertificateMeta struct {
	DNSChallenge           bool   `json:"dns_challenge"`
	DNSProvider            string `json:"dns_provider,omitempty"`
	DNSProviderCredentials string `json:"dns_provider_credentials,omitempty"`
	PropagationSeconds     *int   `json:"propagation_seconds,omitempty"`
}

// CertificateResponse represents the NPM certificate response
type CertificateResponse struct {
	ID          int      `json:"id"`
	DomainNames []string `json:"domain_names"`
	ExpiresOn   string   `json:"expires_on"`
	Provider    string   `json:"provider"`
}

// NewClient creates a new NPM API client
func NewClient(baseURL, email, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		email:    email,
		password: password,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// doRequest performs an HTTP request with authentication and JSON handling
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}

// readErrorBody reads the response body for error messages
func readErrorBody(resp *http.Response) string {
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

// Login authenticates with the NPM API and stores the token
func (c *Client) Login(ctx context.Context) error {
	body := map[string]string{
		"identity": c.email,
		"secret":   c.password,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/tokens", body)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	if result.Token == "" {
		return fmt.Errorf("no token in login response")
	}

	c.token = result.Token
	return nil
}

// CreateProxyHost creates a new proxy host and returns its ID
func (c *Client) CreateProxyHost(ctx context.Context, host ProxyHost) (int, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/nginx/proxy-hosts", host)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("create proxy host failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.ID, nil
}

// UpdateProxyHost updates an existing proxy host
func (c *Client) UpdateProxyHost(ctx context.Context, id int, host ProxyHost) (int, error) {
	path := fmt.Sprintf("/api/nginx/proxy-hosts/%d", id)
	resp, err := c.doRequest(ctx, http.MethodPut, path, host)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("update proxy host failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	return id, nil
}

// DeleteProxyHost deletes a proxy host by ID
func (c *Client) DeleteProxyHost(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/nginx/proxy-hosts/%d", id)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete proxy host failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	return nil
}

// CreateLetsEncryptCertificate creates a Let's Encrypt certificate using DNS challenge
func (c *Client) CreateLetsEncryptCertificate(ctx context.Context, domains []string, dnsProvider, dnsCredentials string, propagationSeconds *int) (*CertificateResponse, error) {
	reqBody := CertificateRequest{
		DomainNames: domains,
		Provider:    "letsencrypt",
		Meta: CertificateMeta{
			DNSChallenge:           true,
			DNSProvider:            dnsProvider,
			DNSProviderCredentials: dnsCredentials,
			PropagationSeconds:     propagationSeconds,
		},
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/nginx/certificates", reqBody)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create certificate failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	var certResp CertificateResponse
	if err := json.NewDecoder(resp.Body).Decode(&certResp); err != nil {
		return nil, fmt.Errorf("failed to decode certificate response: %w", err)
	}

	return &certResp, nil
}

// GetCertificate retrieves a certificate by ID, returns nil if not found
func (c *Client) GetCertificate(ctx context.Context, id int) (*CertificateResponse, error) {
	path := fmt.Sprintf("/api/nginx/certificates/%d", id)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get certificate failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	var certResp CertificateResponse
	if err := json.NewDecoder(resp.Body).Decode(&certResp); err != nil {
		return nil, fmt.Errorf("failed to decode certificate response: %w", err)
	}

	return &certResp, nil
}

// DeleteCertificate deletes a certificate by ID
func (c *Client) DeleteCertificate(ctx context.Context, id int) error {
	path := fmt.Sprintf("/api/nginx/certificates/%d", id)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete certificate failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	return nil
}

// ListCertificates retrieves all certificates from NPM
func (c *Client) ListCertificates(ctx context.Context) ([]CertificateResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/nginx/certificates", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list certificates failed with status %d: %s", resp.StatusCode, readErrorBody(resp))
	}

	var certs []CertificateResponse
	if err := json.NewDecoder(resp.Body).Decode(&certs); err != nil {
		return nil, fmt.Errorf("failed to decode certificates response: %w", err)
	}

	return certs, nil
}

// FindCertificateByDomains finds an existing certificate that matches the given domains
// Returns nil if no matching certificate is found
func (c *Client) FindCertificateByDomains(ctx context.Context, domains []string) (*CertificateResponse, error) {
	certs, err := c.ListCertificates(ctx)
	if err != nil {
		return nil, err
	}

	// Create a map for quick domain lookup
	requestedDomains := make(map[string]bool)
	for _, d := range domains {
		requestedDomains[d] = true
	}

	// Find a certificate that covers all requested domains
	for _, cert := range certs {
		if domainsMatch(cert.DomainNames, requestedDomains) {
			return &cert, nil
		}
	}

	return nil, nil
}

// domainsMatch checks if cert domains match the requested domains exactly
func domainsMatch(certDomains []string, requestedDomains map[string]bool) bool {
	if len(certDomains) != len(requestedDomains) {
		return false
	}
	for _, d := range certDomains {
		if !requestedDomains[d] {
			return false
		}
	}
	return true
}
