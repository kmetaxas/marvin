package kafka

import (
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderName(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})
	assert.Equal(t, "kafka", p.Name())
}

func TestProviderCapabilities(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})
	caps := p.Capabilities()
	require.NotEmpty(t, caps)
	assert.Equal(t, "kafka", caps[0].Provider)
}

func TestProviderGetTask(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})

	tk, ok := p.GetTask("kafka.topic.list")
	require.True(t, ok)
	assert.Equal(t, "kafka.topic.list", tk.Name())

	_, ok = p.GetTask("kafka.unknown.task")
	assert.False(t, ok)
}

func TestProviderIsEnabled(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})
	assert.True(t, p.IsEnabled(nil))
}

func TestProviderCurrentClient(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})
	client := p.CurrentClient()
	assert.NotNil(t, client)
}

func TestProviderUpdateConfig(t *testing.T) {
	t.Parallel()

	// Start with a basic config.
	cfg := config.KafkaConfig{
		BootstrapServers: []string{"localhost:9092"},
		DialTimeout:      5 * time.Second,
	}
	p := NewProvider(cfg)

	// Verify initial client exists.
	client1 := p.CurrentClient()
	require.NotNil(t, client1)

	// Update with identical config - client should stay the same.
	p.UpdateConfig("", map[string]any{
		"bootstrap_servers": []string{"localhost:9092"},
		"dial_timeout":      5 * time.Second,
	})
	client2 := p.CurrentClient()
	require.NotNil(t, client2)
	assert.Equal(t, client1, client2)

	// Update with different bootstrap_servers - client should change.
	p.UpdateConfig("", map[string]any{
		"bootstrap_servers": []string{"newhost:9092"},
		"dial_timeout":      5 * time.Second,
	})
	client3 := p.CurrentClient()
	require.NotNil(t, client3)
	assert.NotEqual(t, client1, client3)
}

func TestProviderUpdateConfigParseFields(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})

	p.UpdateConfig("", map[string]any{
		"bootstrap_servers": []string{"host1:9092", "host2:9092"},
		"tls": map[string]any{
			"enabled":              true,
			"ca_file":              "/ca.crt",
			"cert_file":            "/client.crt",
			"key_file":             "/client.key",
			"insecure_skip_verify": true,
		},
		"sasl": map[string]any{
			"mechanism": "SCRAM-SHA-256",
			"username":  "user",
			"password":  "pass",
		},
		"dial_timeout":     "10s",
		"request_timeout":  "20s",
		"metadata_max_age": "30s",
		"guardrails": map[string]any{
			"max_topics_per_request":     50,
			"max_partitions_per_request": 100,
			"max_consumer_groups":        10,
			"max_records_per_sample":     1000,
			"max_response_bytes":         1024,
			"max_query_duration":         "5s",
		},
	})

	client := p.CurrentClient()
	require.NotNil(t, client)
}

func TestParseKafkaConfigInlineCredentialFields(t *testing.T) {
	t.Parallel()

	cfg, err := parseKafkaConfig(config.KafkaConfig{}, map[string]any{
		"bootstrap_servers": []string{"localhost:9092"},
		"tls": map[string]any{
			"enabled":   true,
			"ca_data":   "ca-pem",
			"cert_data": "cert-pem",
			"key_data":  "key-pem",
		},
		"sasl": map[string]any{
			"mechanism":             "GSSAPI",
			"username":              "alice",
			"keytab_data":           "keytab-bytes",
			"krb5_conf_data":        "krb5-data",
			"krb5_conf_path":        "/custom/krb5.conf",
			"keytab_path":           "/custom/alice.keytab",
			"kerberos_realm":        "EXAMPLE.COM",
			"kerberos_service_name": "kafka",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "ca-pem", cfg.TLS.CAData)
	assert.Equal(t, "cert-pem", cfg.TLS.CertData)
	assert.Equal(t, "key-pem", cfg.TLS.KeyData)
	assert.Equal(t, "keytab-bytes", cfg.SASL.KeytabData)
	assert.Equal(t, "krb5-data", cfg.SASL.Krb5ConfData)
	assert.Equal(t, "/custom/krb5.conf", cfg.SASL.Krb5ConfPath)
	assert.Equal(t, "/custom/alice.keytab", cfg.SASL.KeytabPath)
}

func TestParseKafkaConfigInlineCredentialDotFields(t *testing.T) {
	t.Parallel()

	cfg, err := parseKafkaConfig(config.KafkaConfig{}, map[string]any{
		"bootstrap_servers":   []string{"localhost:9092"},
		"tls.ca_data":         "ca-pem",
		"tls.cert_data":       "cert-pem",
		"tls.key_data":        "key-pem",
		"sasl.keytab_data":    "keytab-bytes",
		"sasl.krb5_conf_data": "krb5-data",
		"sasl.krb5_conf_path": "/custom/krb5.conf",
	})
	require.NoError(t, err)

	assert.Equal(t, "ca-pem", cfg.TLS.CAData)
	assert.Equal(t, "cert-pem", cfg.TLS.CertData)
	assert.Equal(t, "key-pem", cfg.TLS.KeyData)
	assert.Equal(t, "keytab-bytes", cfg.SASL.KeytabData)
	assert.Equal(t, "krb5-data", cfg.SASL.Krb5ConfData)
	assert.Equal(t, "/custom/krb5.conf", cfg.SASL.Krb5ConfPath)
}

func TestProviderUpdateConfigBootstrapServersAnySlice(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})

	// Test with []any slice.
	p.UpdateConfig("", map[string]any{
		"bootstrap_servers": []any{"host1:9092", "host2:9092"},
	})

	client := p.CurrentClient()
	require.NotNil(t, client)
}

func TestProviderUpdateConfigDurationTypes(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})

	// Test duration as int (seconds).
	p.UpdateConfig("", map[string]any{
		"bootstrap_servers": []string{"localhost:9092"},
		"dial_timeout":      15,
	})
	require.NotNil(t, p.CurrentClient())

	// Test duration as float64.
	p.UpdateConfig("", map[string]any{
		"bootstrap_servers": []string{"localhost:9092"},
		"dial_timeout":      float64(15),
	})
	require.NotNil(t, p.CurrentClient())

	// Test duration as time.Duration.
	p.UpdateConfig("", map[string]any{
		"bootstrap_servers": []string{"localhost:9092"},
		"dial_timeout":      15 * time.Second,
	})
	require.NotNil(t, p.CurrentClient())
}

func TestProviderTasksWiredToProvider(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.KafkaConfig{BootstrapServers: []string{"localhost:9092"}})

	tk, ok := p.GetTask("kafka.topic.list")
	require.True(t, ok)

	task := tk.(*topicListTask)
	assert.Equal(t, p, task.provider)
}
