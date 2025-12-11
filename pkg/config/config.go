package config

import (
	"os"

	corev1 "k8s.io/api/core/v1"
)

const (
	// DefaultOperatorNamespace is the default namespace for operator configuration
	DefaultOperatorNamespace = "npmk-operator-system"

	// ConfigMapName is the name of the ConfigMap containing NPM configuration
	ConfigMapName = "npmko-config"

	// CredentialsSecretName is the name of the Secret containing NPM password
	CredentialsSecretName = "npmko-credentials"
)

// Config holds NPM connection configuration
type Config struct {
	NPMURL      string
	NPMEmail    string
	NPMPassword string
}

// GetOperatorNamespace returns the operator namespace from env or default
func GetOperatorNamespace() string {
	if ns := os.Getenv("OPERATOR_NAMESPACE"); ns != "" {
		return ns
	}
	return DefaultOperatorNamespace
}

// ParseConfigMap extracts configuration from a ConfigMap
func ParseConfigMap(cm *corev1.ConfigMap) *Config {
	cfg := &Config{}

	if url, ok := cm.Data["npm-url"]; ok {
		cfg.NPMURL = url
	}
	if email, ok := cm.Data["npm-email"]; ok {
		cfg.NPMEmail = email
	}
	// Support legacy password in ConfigMap (not recommended)
	if password, ok := cm.Data["npm-password"]; ok {
		cfg.NPMPassword = password
	}

	return cfg
}

// ParseCredentialsSecret extracts password from a Secret
func ParseCredentialsSecret(secret *corev1.Secret) string {
	if password, ok := secret.Data["password"]; ok {
		return string(password)
	}
	return ""
}

// Merge combines two configs, with other taking precedence for non-empty values
func (c *Config) Merge(other *Config) {
	if other.NPMURL != "" {
		c.NPMURL = other.NPMURL
	}
	if other.NPMEmail != "" {
		c.NPMEmail = other.NPMEmail
	}
	if other.NPMPassword != "" {
		c.NPMPassword = other.NPMPassword
	}
}

// Validate checks if required configuration is present
func (c *Config) Validate() error {
	if c.NPMURL == "" {
		return &ConfigError{Field: "npm-url", Message: "NPM URL is required"}
	}
	if c.NPMEmail == "" {
		return &ConfigError{Field: "npm-email", Message: "NPM email is required"}
	}
	if c.NPMPassword == "" {
		return &ConfigError{Field: "npm-password", Message: "NPM password is required"}
	}
	return nil
}

// ConfigError represents a configuration validation error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}
