package config

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetOperatorNamespace(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "default namespace when env not set",
			envValue: "",
			want:     DefaultOperatorNamespace,
		},
		{
			name:     "custom namespace from env",
			envValue: "custom-namespace",
			want:     "custom-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("OPERATOR_NAMESPACE", tt.envValue)
			} else {
				t.Setenv("OPERATOR_NAMESPACE", "")
			}

			got := GetOperatorNamespace()
			if got != tt.want {
				t.Errorf("GetOperatorNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseConfigMap(t *testing.T) {
	tests := []struct {
		name string
		cm   *corev1.ConfigMap
		want *Config
	}{
		{
			name: "full config",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "npm-config"},
				Data: map[string]string{
					"npm-url":      "http://npm:81",
					"npm-email":    "admin@example.com",
					"npm-password": "secret",
				},
			},
			want: &Config{
				NPMURL:      "http://npm:81",
				NPMEmail:    "admin@example.com",
				NPMPassword: "secret",
			},
		},
		{
			name: "partial config",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "npm-config"},
				Data: map[string]string{
					"npm-url":   "http://npm:81",
					"npm-email": "admin@example.com",
				},
			},
			want: &Config{
				NPMURL:   "http://npm:81",
				NPMEmail: "admin@example.com",
			},
		},
		{
			name: "empty config",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "npm-config"},
				Data:       map[string]string{},
			},
			want: &Config{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseConfigMap(tt.cm)
			if got.NPMURL != tt.want.NPMURL {
				t.Errorf("ParseConfigMap().NPMURL = %v, want %v", got.NPMURL, tt.want.NPMURL)
			}
			if got.NPMEmail != tt.want.NPMEmail {
				t.Errorf("ParseConfigMap().NPMEmail = %v, want %v", got.NPMEmail, tt.want.NPMEmail)
			}
			if got.NPMPassword != tt.want.NPMPassword {
				t.Errorf("ParseConfigMap().NPMPassword = %v, want %v", got.NPMPassword, tt.want.NPMPassword)
			}
		})
	}
}

func TestParseCredentialsSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
		want   string
	}{
		{
			name: "password present",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "npm-credentials"},
				Data: map[string][]byte{
					"password": []byte("mypassword"),
				},
			},
			want: "mypassword",
		},
		{
			name: "password missing",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "npm-credentials"},
				Data:       map[string][]byte{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCredentialsSecret(tt.secret)
			if got != tt.want {
				t.Errorf("ParseCredentialsSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfigMerge(t *testing.T) {
	tests := []struct {
		name  string
		base  *Config
		other *Config
		want  *Config
	}{
		{
			name: "merge overwrites non-empty values",
			base: &Config{
				NPMURL:      "http://old:81",
				NPMEmail:    "old@example.com",
				NPMPassword: "oldpass",
			},
			other: &Config{
				NPMURL:      "http://new:81",
				NPMEmail:    "new@example.com",
				NPMPassword: "newpass",
			},
			want: &Config{
				NPMURL:      "http://new:81",
				NPMEmail:    "new@example.com",
				NPMPassword: "newpass",
			},
		},
		{
			name: "merge preserves base for empty other values",
			base: &Config{
				NPMURL:      "http://old:81",
				NPMEmail:    "old@example.com",
				NPMPassword: "oldpass",
			},
			other: &Config{
				NPMURL: "http://new:81",
			},
			want: &Config{
				NPMURL:      "http://new:81",
				NPMEmail:    "old@example.com",
				NPMPassword: "oldpass",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Merge(tt.other)
			if tt.base.NPMURL != tt.want.NPMURL {
				t.Errorf("Merge().NPMURL = %v, want %v", tt.base.NPMURL, tt.want.NPMURL)
			}
			if tt.base.NPMEmail != tt.want.NPMEmail {
				t.Errorf("Merge().NPMEmail = %v, want %v", tt.base.NPMEmail, tt.want.NPMEmail)
			}
			if tt.base.NPMPassword != tt.want.NPMPassword {
				t.Errorf("Merge().NPMPassword = %v, want %v", tt.base.NPMPassword, tt.want.NPMPassword)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				NPMURL:      "http://npm:81",
				NPMEmail:    "admin@example.com",
				NPMPassword: "password",
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			config: &Config{
				NPMEmail:    "admin@example.com",
				NPMPassword: "password",
			},
			wantErr: true,
			errMsg:  "npm-url",
		},
		{
			name: "missing email",
			config: &Config{
				NPMURL:      "http://npm:81",
				NPMPassword: "password",
			},
			wantErr: true,
			errMsg:  "npm-email",
		},
		{
			name: "missing password",
			config: &Config{
				NPMURL:   "http://npm:81",
				NPMEmail: "admin@example.com",
			},
			wantErr: true,
			errMsg:  "npm-password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				configErr, ok := err.(*ConfigError)
				if !ok {
					t.Errorf("Validate() error type = %T, want *ConfigError", err)
				} else if configErr.Field != tt.errMsg {
					t.Errorf("Validate() error field = %v, want %v", configErr.Field, tt.errMsg)
				}
			}
		})
	}
}
