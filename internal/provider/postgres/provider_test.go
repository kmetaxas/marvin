package postgres

import (
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.PostgresConfig{})
	assert.Equal(t, "postgres", p.Name())
	assert.Len(t, p.Capabilities(), 14)
	assert.True(t, p.IsEnabled(nil))

	expected := []string{
		"postgres.connection.ping",
		"postgres.connection.stats",
		"postgres.instance.info",
		"postgres.session.list",
		"postgres.query.top",
		"postgres.table.stats",
		"postgres.table.vacuum_status",
		"postgres.lock.blocking",
		"postgres.transaction.long_running",
		"postgres.maintenance.progress",
		"postgres.replication.status",
		"postgres.replication.slots",
		"postgres.wal.archiver",
		"postgres.storage.largest_relations",
	}
	for _, name := range expected {
		found, ok := p.GetTask(name)
		require.True(t, ok, "expected task %q to exist", name)
		assert.NotNil(t, found)
		assert.Equal(t, name, found.Name())
	}
}

func TestProviderIsConfiguredEmpty(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.PostgresConfig{})
	assert.False(t, p.IsConfigured())
}

func TestProviderIsConfiguredWithHost(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.PostgresConfig{Host: "localhost"})
	assert.True(t, p.IsConfigured())
}

func TestProviderIsConfiguredWithDSN(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.PostgresConfig{DSN: "postgres://user:pass@localhost/db"})
	assert.True(t, p.IsConfigured())
}

func TestProviderCurrentClientNilInitially(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.PostgresConfig{})
	client := p.CurrentClient()
	assert.Nil(t, client)
}

func TestProviderDefensiveCopy(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.PostgresConfig{Host: "localhost"})
	caps := p.Capabilities()
	require.Len(t, caps, 14)
	caps[0].Name = "tampered"
	assert.NotEqual(t, "tampered", p.Capabilities()[0].Name)
}

func TestProviderGetTaskUnknown(t *testing.T) {
	t.Parallel()
	p := NewProvider(config.PostgresConfig{})
	_, ok := p.GetTask("postgres.unknown.task")
	assert.False(t, ok)
}

func TestCapabilityDescription(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Test connectivity to a PostgreSQL database and return latency and version information.", capabilityDescription("postgres.connection.ping"))
	assert.Equal(t, "PostgreSQL capability", capabilityDescription("postgres.unknown.thing"))
}

func TestParsePostgresConfigMap(t *testing.T) {
	t.Parallel()

	cfg, err := parsePostgresConfigMap(config.PostgresConfig{}, map[string]any{
		"host":              "localhost",
		"port":              5432,
		"database":          "mydb",
		"user":              "myuser",
		"password":          "mypass",
		"dsn":               "postgres://u:p@h/d",
		"connect_timeout":   "5s",
		"max_conns":         10,
		"min_conns":         2,
		"conn_max_lifetime": "30m",
		"tls": map[string]any{
			"enabled":              true,
			"ca_file":              "/ca.crt",
			"cert_file":            "/client.crt",
			"key_file":             "/client.key",
			"insecure_skip_verify": true,
		},
		"guardrails": map[string]any{
			"max_rows":              1000,
			"statement_timeout":     "30s",
			"max_result_size_bytes": 1048576,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 5432, cfg.Port)
	assert.Equal(t, "mydb", cfg.Database)
	assert.Equal(t, "myuser", cfg.User)
	assert.Equal(t, "mypass", cfg.Password)
	assert.Equal(t, "postgres://u:p@h/d", cfg.DSN)
	assert.Equal(t, int32(10), cfg.MaxConns)
	assert.Equal(t, int32(2), cfg.MinConns)
	assert.True(t, cfg.TLS.Enabled)
	assert.Equal(t, "/ca.crt", cfg.TLS.CAFile)
	assert.Equal(t, "/client.crt", cfg.TLS.CertFile)
	assert.Equal(t, "/client.key", cfg.TLS.KeyFile)
	assert.True(t, cfg.TLS.InsecureSkipVerify)
	assert.Equal(t, 1000, cfg.Guardrails.MaxRows)
	assert.Equal(t, 1048576, cfg.Guardrails.MaxResultSizeBytes)
}

func TestParsePostgresConfigMapDotFields(t *testing.T) {
	t.Parallel()

	cfg, err := parsePostgresConfigMap(config.PostgresConfig{}, map[string]any{
		"host":                         "localhost",
		"tls.enabled":                  true,
		"tls.ca_data":                  "ca-pem",
		"tls.cert_data":                "cert-pem",
		"tls.key_data":                 "key-pem",
		"guardrails.max_rows":          500,
		"guardrails.statement_timeout": "10s",
	})
	require.NoError(t, err)
	assert.True(t, cfg.TLS.Enabled)
	assert.Equal(t, "ca-pem", cfg.TLS.CAData)
	assert.Equal(t, "cert-pem", cfg.TLS.CertData)
	assert.Equal(t, "key-pem", cfg.TLS.KeyData)
	assert.Equal(t, 500, cfg.Guardrails.MaxRows)
}

func TestProviderTasksImplementTask(t *testing.T) {
	t.Parallel()
	var _ task.Task = (*connectionPingTask)(nil)
	var _ task.Task = (*connectionStatsTask)(nil)
	var _ task.Task = (*instanceInfoTask)(nil)
	var _ task.Task = (*sessionListTask)(nil)
	var _ task.Task = (*queryTopTask)(nil)
	var _ task.Task = (*tableStatsTask)(nil)
	var _ task.Task = (*tableVacuumStatusTask)(nil)
	var _ task.Task = (*lockBlockingTask)(nil)
	var _ task.Task = (*transactionLongRunningTask)(nil)
	var _ task.Task = (*maintenanceProgressTask)(nil)
	var _ task.Task = (*replicationStatusTask)(nil)
	var _ task.Task = (*replicationSlotsTask)(nil)
	var _ task.Task = (*walArchiverTask)(nil)
	var _ task.Task = (*storageLargestRelationsTask)(nil)
}
