package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentmetadata "github.com/marvin-agent/marvin/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAndValidate(t *testing.T) {
	t.Parallel()

	path := filepath.Join("testdata", "valid.yaml")
	cfg, err := Load(path)
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())

	assert.Equal(t, "cp.example.com:443", cfg.ControlPlane.Address)
	assert.Equal(t, "marvin-us-east-1", cfg.Metadata.Name)
	assert.Equal(t, []string{"network.dns.lookup", "network.icmp.echo_request"}, cfg.Capabilities.Enabled)

	assert.Equal(t, "marvin-abc123", cfg.AgentID)
	assert.Equal(t, "org-reg-key", cfg.RegistrationKey)
	assert.Equal(t, []string{"env:production"}, cfg.Labels)

	method, methodCfg, err := cfg.Authentication.AuthMethod()
	require.NoError(t, err)
	assert.Equal(t, "secret_key", method)
	assert.Equal(t, SecretKeyAuth{Key: "super-secret-token"}, methodCfg)
}

func TestValidateFailures(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tests := []struct {
		name    string
		cfg     func() Config
		wantErr string
	}{
		{
			name: "missing control plane address",
			cfg: func() Config {
				cfg := validConfig()
				cfg.ControlPlane.Address = ""
				return cfg
			},
			wantErr: "control_plane.address is required",
		},
		{
			name: "missing agent id",
			cfg: func() Config {
				cfg := validConfig()
				cfg.AgentID = ""
				return cfg
			},
			wantErr: "agent_id is required",
		},
		{
			name: "multiple auth methods",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Authentication.MTLS = &MTLSAuth{CertFile: "cert.pem", KeyFile: "key.pem"}
				return cfg
			},
			wantErr: "exactly one authentication method must be configured",
		},
		{
			name: "invalid capability name",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Capabilities.Enabled = []string{"network.INVALID.lookup"}
				return cfg
			},
			wantErr: "validate capabilities.enabled entry",
		},
		{
			name: "invalid log level",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Logging.Level = "trace"
				return cfg
			},
			wantErr: "invalid log level",
		},
		{
			name: "ca file missing",
			cfg: func() Config {
				cfg := validConfig()
				cfg.ControlPlane.TLS.CACertFile = filepath.Join(tmpDir, "missing.crt")
				return cfg
			},
			wantErr: "stat ca cert file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg()
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestAzureValidationSkippedWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Azure = AzureConfig{}
	cfg.Capabilities.Enabled = []string{"network.dns.lookup"}
	require.NoError(t, cfg.Validate())
}

func TestKubernetesValidationSkippedWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Kubernetes = KubernetesConfig{KubeconfigPath: filepath.Join(t.TempDir(), "nonexistent")}
	cfg.Capabilities.Enabled = []string{"network.dns.lookup"}
	require.NoError(t, cfg.Validate())
}

func TestPrometheusValidationSkippedWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Prometheus = PrometheusConfig{}
	cfg.Capabilities.Enabled = []string{"network.dns.lookup"}
	require.NoError(t, cfg.Validate())
}

func TestHasPrometheusCapabilityEnabledWithPattern(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Capabilities.Enabled = []string{"prometheus.*"}
	assert.True(t, cfg.hasPrometheusCapabilityEnabled())
}

func TestPrometheusConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     PrometheusConfig
		wantErr string
	}{
		{
			name: "valid none auth",
			cfg:  PrometheusConfig{URL: "http://localhost:9090"},
		},
		{
			name: "valid basic auth",
			cfg:  PrometheusConfig{URL: "http://localhost:9090", Auth: PrometheusAuthConfig{Type: "basic", Username: "u", Password: "p"}},
		},
		{
			name: "valid bearer auth",
			cfg:  PrometheusConfig{URL: "http://localhost:9090", Auth: PrometheusAuthConfig{Type: "bearer", Token: "t"}},
		},
		{
			name: "valid header auth",
			cfg:  PrometheusConfig{URL: "http://localhost:9090", Auth: PrometheusAuthConfig{Type: "header", HeaderName: "X-Api-Key", HeaderValue: "v"}},
		},
		{
			name:    "missing url",
			cfg:     PrometheusConfig{},
			wantErr: "prometheus.url is required",
		},
		{
			name:    "basic missing username",
			cfg:     PrometheusConfig{URL: "http://localhost:9090", Auth: PrometheusAuthConfig{Type: "basic"}},
			wantErr: "prometheus.auth.username is required",
		},
		{
			name:    "bearer missing token",
			cfg:     PrometheusConfig{URL: "http://localhost:9090", Auth: PrometheusAuthConfig{Type: "bearer"}},
			wantErr: "prometheus.auth.token is required",
		},
		{
			name:    "header missing name",
			cfg:     PrometheusConfig{URL: "http://localhost:9090", Auth: PrometheusAuthConfig{Type: "header"}},
			wantErr: "prometheus.auth.header_name is required",
		},
		{
			name:    "unsupported auth type",
			cfg:     PrometheusConfig{URL: "http://localhost:9090", Auth: PrometheusAuthConfig{Type: "oauth2"}},
			wantErr: "unsupported auth type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestValidateWithPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     func() Config
		wantErr string
	}{
		{
			name: "valid pattern all",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Capabilities.Enabled = []string{"*"}
				return cfg
			},
			wantErr: "",
		},
		{
			name: "valid pattern provider prefix",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Capabilities.Enabled = []string{"kubernetes.*"}
				return cfg
			},
			wantErr: "",
		},
		{
			name: "valid pattern suffix",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Capabilities.Enabled = []string{"*.list"}
				return cfg
			},
			wantErr: "",
		},
		{
			name: "invalid pattern characters",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Capabilities.Enabled = []string{"kubernetes.!"}
				return cfg
			},
			wantErr: "validate capabilities.enabled entry",
		},
		{
			name: "invalid pattern only wildcards",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Capabilities.Enabled = []string{"***"}
				return cfg
			},
			wantErr: "validate capabilities.enabled pattern",
		},
		{
			name: "mixed exact and pattern",
			cfg: func() Config {
				cfg := validConfig()
				cfg.Capabilities.Enabled = []string{"network.dns.lookup", "kubernetes.*"}
				return cfg
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.cfg()
			err := cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestHasKubernetesCapabilityEnabledWithPattern(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Capabilities.Enabled = []string{"kubernetes.*"}
	assert.True(t, cfg.hasKubernetesCapabilityEnabled())
}

func TestHasAzureCapabilityEnabledWithPattern(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Capabilities.Enabled = []string{"azure.*"}
	assert.True(t, cfg.hasAzureCapabilityEnabled())
}

func TestTLSConfigIsEnabled(t *testing.T) {
	t.Parallel()

	assert.True(t, TLSConfig{}.IsEnabled(), "default (nil Enabled) should be true")
	assert.False(t, TLSConfig{Enabled: boolPtr(false)}.IsEnabled())
	assert.True(t, TLSConfig{Enabled: boolPtr(true)}.IsEnabled())
}

func TestGraylogValidationSkippedWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Graylog = GraylogConfig{}
	cfg.Capabilities.Enabled = []string{"network.dns.lookup"}
	require.NoError(t, cfg.Validate())
}

func TestHasGraylogCapabilityEnabledWithPattern(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Capabilities.Enabled = []string{"log.graylog.*"}
	assert.True(t, cfg.hasGraylogCapabilityEnabled())
}

func TestGraylogConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     GraylogConfig
		wantErr string
	}{
		{
			name: "valid none auth",
			cfg:  GraylogConfig{URL: "https://graylog.example.com:9000/api"},
		},
		{
			name: "valid token auth",
			cfg:  GraylogConfig{URL: "https://graylog.example.com:9000/api", Auth: GraylogAuthConfig{Type: "token", Token: "abc123"}},
		},
		{
			name: "valid basic auth",
			cfg:  GraylogConfig{URL: "https://graylog.example.com:9000/api", Auth: GraylogAuthConfig{Type: "basic", Username: "admin", Password: "secret"}},
		},
		{
			name:    "missing url",
			cfg:     GraylogConfig{},
			wantErr: "graylog.url is required",
		},
		{
			name:    "http scheme",
			cfg:     GraylogConfig{URL: "http://graylog.example.com:9000/api"},
			wantErr: "scheme must be https",
		},
		{
			name:    "token missing token",
			cfg:     GraylogConfig{URL: "https://graylog.example.com:9000/api", Auth: GraylogAuthConfig{Type: "token"}},
			wantErr: "graylog.auth.token is required",
		},
		{
			name:    "basic missing username",
			cfg:     GraylogConfig{URL: "https://graylog.example.com:9000/api", Auth: GraylogAuthConfig{Type: "basic"}},
			wantErr: "graylog.auth.username is required",
		},
		{
			name:    "unsupported auth type",
			cfg:     GraylogConfig{URL: "https://graylog.example.com:9000/api", Auth: GraylogAuthConfig{Type: "oauth2"}},
			wantErr: "unsupported auth type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func boolPtr(v bool) *bool { return &v }

func TestTLSConfigBuildTLSConfig(t *testing.T) {
	t.Parallel()

	certFile := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(certFile, mustCreateTestCACertPEM(t), 0o600))

	tlsCfg, err := TLSConfig{CACertFile: certFile}.BuildTLSConfig()
	require.NoError(t, err)
	assert.NotNil(t, tlsCfg)
	assert.NotNil(t, tlsCfg.RootCAs)
	assert.False(t, tlsCfg.InsecureSkipVerify)
}

func TestTLSConfigBuildTLSConfigInvalidPEM(t *testing.T) {
	t.Parallel()

	certFile := filepath.Join(t.TempDir(), "ca.crt")
	require.NoError(t, os.WriteFile(certFile, []byte("not a pem"), 0o600))

	_, err := TLSConfig{CACertFile: certFile}.BuildTLSConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pem data")
}

func validConfig() Config {
	return Config{
		AgentID:         "marvin-abc123",
		RegistrationKey: "org-reg-key",
		Labels:          []string{"env:production", "team:platform"},
		ControlPlane:    ControlPlane{Address: "cp.example.com:443"},
		Authentication:  Authentication{SecretKey: &SecretKeyAuth{Key: "super-secret-token"}},
		Metadata: agentmetadata.Metadata{
			Name:             "marvin-us-east-1",
			Cluster:          "prod-us-east-1",
			Region:           "us-east-1",
			Environment:      "production",
			Hostname:         "server1.us-east.example.com",
			LocalIP:          "10.0.0.1",
			Provider:         "aws",
			AvailabilityZone: "us-east-1a",
			VMID:             "i-1234567890abcdef0",
			OS:               "Linux",
			OSVersion:        "Ubuntu 22.04.3 LTS",
			Arch:             "x86_64",
		},
		Capabilities: Capabilities{Enabled: []string{"network.dns.lookup"}},
		Logging:      Logging{Level: "info"},
		Azure: AzureConfig{
			TenantID:       "72f988bf-86f1-41af-91ab-2d7cd011db47",
			ClientID:       "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			ClientSecret:   "azure-client-secret",
			SubscriptionID: "12345678-1234-1234-1234-123456789012",
			ResourceGroup:  "my-rg",
			WorkspaceID:    "ws-1234",
		},
		Prometheus: PrometheusConfig{
			URL: "http://prometheus:9090",
		},
		Kafka: KafkaConfig{
			BootstrapServers: []string{"localhost:9092"},
		},
	}
}

func TestKafkaConfigValidate(t *testing.T) {
	t.Parallel()

	base := func() KafkaConfig {
		return KafkaConfig{
			BootstrapServers: []string{"localhost:9092"},
		}
	}

	tests := []struct {
		name    string
		cfg     KafkaConfig
		wantErr string
	}{
		{
			name: "valid no auth",
			cfg:  base(),
		},
		{
			name: "valid SCRAM",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.SASL = KafkaSASLConfig{Mechanism: "SCRAM-SHA-256", Username: "u", Password: "p"}
				return cfg
			}(),
		},
		{
			name: "valid GSSAPI with keytab_path",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.SASL = KafkaSASLConfig{
					Mechanism: "GSSAPI", Username: "alice", KerberosServiceName: "kafka",
					KerberosRealm: "EXAMPLE.COM", KeytabPath: "/etc/alice.keytab",
				}
				return cfg
			}(),
		},
		{
			name: "valid GSSAPI with keytab_data",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.SASL = KafkaSASLConfig{
					Mechanism: "GSSAPI", Username: "alice", KerberosServiceName: "kafka",
					KerberosRealm: "EXAMPLE.COM", KeytabData: "base64data==",
				}
				return cfg
			}(),
		},
		{
			name: "GSSAPI missing keytab",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.SASL = KafkaSASLConfig{
					Mechanism: "GSSAPI", Username: "alice", KerberosServiceName: "kafka",
					KerberosRealm: "EXAMPLE.COM",
				}
				return cfg
			}(),
			wantErr: "kafka.sasl.keytab_path or kafka.sasl.keytab_data is required",
		},
		{
			name: "GSSAPI missing service_name",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.SASL = KafkaSASLConfig{
					Mechanism: "GSSAPI", Username: "alice", KerberosRealm: "EXAMPLE.COM",
					KeytabPath: "/etc/alice.keytab",
				}
				return cfg
			}(),
			wantErr: "kafka.sasl.kerberos_service_name is required",
		},
		{
			name: "GSSAPI missing realm",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.SASL = KafkaSASLConfig{
					Mechanism: "GSSAPI", Username: "alice", KerberosServiceName: "kafka",
					KeytabPath: "/etc/alice.keytab",
				}
				return cfg
			}(),
			wantErr: "kafka.sasl.kerberos_realm is required",
		},
		{
			name: "TLS enabled missing CA",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.TLS = KafkaTLSConfig{Enabled: true}
				return cfg
			}(),
			wantErr: "kafka.tls.ca_file or kafka.tls.ca_data is required",
		},
		{
			name: "TLS with CA data only",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.TLS = KafkaTLSConfig{Enabled: true, CAData: "base64ca=="}
				return cfg
			}(),
		},
		{
			name: "TLS client cert missing key",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.TLS = KafkaTLSConfig{Enabled: true, CAData: "base64ca==", CertData: "base64cert=="}
				return cfg
			}(),
			wantErr: "kafka.tls client certificate requires cert_file/cert_data and key_file/key_data",
		},
		{
			name: "TLS valid client cert inline",
			cfg: func() KafkaConfig {
				cfg := base()
				cfg.TLS = KafkaTLSConfig{Enabled: true, CAData: "base64ca==", CertData: "base64cert==", KeyData: "base64key=="}
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func mustCreateTestCACertPEM(t *testing.T) []byte {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "marvin-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}
