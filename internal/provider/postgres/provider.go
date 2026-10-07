package postgres

import (
	"sync"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

// Provider is the runtime-configurable PostgreSQL provider.
type Provider struct {
	mu    sync.RWMutex
	state *autoconfig.State[config.PostgresConfig, PostgresClient]
	base  provider.BaseProvider
}

// NewProvider creates a new PostgreSQL provider with the given initial config.
func NewProvider(cfg config.PostgresConfig) *Provider {
	p := &Provider{}

	var client PostgresClient
	if cfg.Host != "" || cfg.DSN != "" {
		client = newPostgresClient(cfg)
	}
	p.state = autoconfig.NewState(&p.mu, cfg, client)

	tasks := []task.Task{
		&connectionPingTask{provider: p},
		&connectionStatsTask{provider: p},
		&instanceInfoTask{provider: p},
		&sessionListTask{provider: p},
		&queryTopTask{provider: p},
		&tableStatsTask{provider: p},
		&tableVacuumStatusTask{provider: p},
		&lockBlockingTask{provider: p},
		&transactionLongRunningTask{provider: p},
		&maintenanceProgressTask{provider: p},
		&replicationStatusTask{provider: p},
		&replicationSlotsTask{provider: p},
		&walArchiverTask{provider: p},
		&storageLargestRelationsTask{provider: p},
	}

	capabilities := make([]capability.Capability, 0, len(tasks))
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		capabilities = append(capabilities, capability.Capability{
			Name:                 t.Name(),
			Version:              "v1",
			Description:          capabilityDescription(t.Name()),
			Provider:             "postgres",
			ParametersJSONSchema: t.JSONSchema(),
			UseCases:             capabilityUseCases(t.Name()),
			Tags:                 capabilityTags(t.Name()),
		})
		taskMap[t.Name()] = t
	}

	p.base = provider.BaseProvider{
		ProviderName:         "postgres",
		ProviderCapabilities: capabilities,
		Tasks:                taskMap,
	}

	return p
}

// Name returns the provider name.
func (p *Provider) Name() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.base.ProviderName
}

// Capabilities returns a defensive copy of the provider capabilities.
func (p *Provider) Capabilities() []capability.Capability {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]capability.Capability, len(p.base.ProviderCapabilities))
	copy(out, p.base.ProviderCapabilities)
	return out
}

// IsEnabled always returns true for the PostgreSQL provider.
func (p *Provider) IsEnabled(config map[string]any) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.base.EnabledFunc != nil {
		return p.base.EnabledFunc(config)
	}
	return true
}

// GetTask looks up a task by capability name.
func (p *Provider) GetTask(name string) (task.Task, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.base.Tasks == nil {
		return nil, false
	}
	t, ok := p.base.Tasks[name]
	return t, ok
}

// CurrentClient returns the current PostgreSQL client under a read lock.
func (p *Provider) CurrentClient() PostgresClient {
	if p == nil || p.state == nil {
		return nil
	}
	return p.state.Client()
}

// IsConfigured returns whether the provider has enough configuration to connect.
func (p *Provider) IsConfigured() bool {
	if p == nil || p.state == nil {
		return false
	}
	cfg := p.state.Config()
	return cfg.Host != "" || cfg.DSN != ""
}

// UpdateConfig parses cfg fields and, if the configuration has changed,
// updates the stored config and recreates the PostgreSQL client.
func (p *Provider) UpdateConfig(capabilityName string, cfg map[string]any) {
	_ = capabilityName

	_, _, _ = autoconfig.Apply(p.state, cfg, autoconfig.Options[config.PostgresConfig, PostgresClient]{
		Parse: parsePostgresConfigMap,
		Build: func(cfg config.PostgresConfig) (PostgresClient, error) {
			return newPostgresClient(cfg), nil
		},
		Close: func(client PostgresClient) {
			if client != nil {
				client.Close()
			}
		},
	})
}

func parsePostgresConfigMap(_ config.PostgresConfig, cfg map[string]any) (config.PostgresConfig, error) {
	out := config.PostgresConfig{}

	if v, ok := autoconfig.StringOK(cfg, "host"); ok {
		out.Host = v
	}
	if v, ok := autoconfig.IntOK(cfg, "port"); ok {
		out.Port = v
	}
	if v, ok := autoconfig.StringOK(cfg, "database"); ok {
		out.Database = v
	}
	if v, ok := autoconfig.StringOK(cfg, "user"); ok {
		out.User = v
	}
	if v, ok := autoconfig.StringOK(cfg, "password"); ok {
		out.Password = v
	}
	if v, ok := autoconfig.StringOK(cfg, "dsn"); ok {
		out.DSN = v
	}
	if d, ok := autoconfig.DurationOK(cfg, "connect_timeout"); ok {
		out.ConnectTimeout = d
	}
	if v, ok := autoconfig.IntOK(cfg, "max_conns"); ok {
		out.MaxConns = int32(v)
	}
	if v, ok := autoconfig.IntOK(cfg, "min_conns"); ok {
		out.MinConns = int32(v)
	}
	if d, ok := autoconfig.DurationOK(cfg, "conn_max_lifetime"); ok {
		out.ConnMaxLifetime = d
	}

	out.TLS, _ = parsePostgresTLSConfig(config.PostgresTLSConfig{}, cfg)
	out.Guardrails, _ = parsePostgresGuardrails(config.PostgresGuardrails{}, cfg)

	return out, nil
}

func parsePostgresTLSConfig(_ config.PostgresTLSConfig, cfg map[string]any) (config.PostgresTLSConfig, error) {
	out := config.PostgresTLSConfig{}

	if v, ok := autoconfig.BoolOK(cfg, "tls.enabled"); ok {
		out.Enabled = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.ca_file"); ok {
		out.CAFile = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.ca_data"); ok {
		out.CAData = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.cert_file"); ok {
		out.CertFile = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.cert_data"); ok {
		out.CertData = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.key_file"); ok {
		out.KeyFile = v
	}
	if v, ok := autoconfig.StringOK(cfg, "tls.key_data"); ok {
		out.KeyData = v
	}
	if v, ok := autoconfig.BoolOK(cfg, "tls.insecure_skip_verify"); ok {
		out.InsecureSkipVerify = v
	}

	return out, nil
}

func parsePostgresGuardrails(_ config.PostgresGuardrails, cfg map[string]any) (config.PostgresGuardrails, error) {
	out := config.PostgresGuardrails{}

	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_rows"); ok {
		out.MaxRows = v
	}
	if d, ok := autoconfig.DurationOK(cfg, "guardrails.statement_timeout"); ok {
		out.StatementTimeout = d
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_result_size_bytes"); ok {
		out.MaxResultSizeBytes = v
	}

	return out, nil
}

func capabilityDescription(name string) string {
	switch name {
	case "postgres.connection.ping":
		return "Test connectivity to a PostgreSQL database and return latency and version information."
	case "postgres.connection.stats":
		return "Return connection pool statistics and active connection counts."
	case "postgres.instance.info":
		return "Return PostgreSQL instance version, settings, and uptime."
	case "postgres.session.list":
		return "List active sessions with query, state, and duration."
	case "postgres.query.top":
		return "Return top queries by execution time or frequency."
	case "postgres.table.stats":
		return "Return table row counts, sizes, and index usage."
	case "postgres.table.vacuum_status":
		return "Return vacuum and analyze status for tables."
	case "postgres.lock.blocking":
		return "Identify blocking and waiting lock relationships."
	case "postgres.transaction.long_running":
		return "Find transactions that have been running longer than a threshold."
	case "postgres.maintenance.progress":
		return "Show progress of running VACUUM, CREATE INDEX, or other maintenance operations."
	case "postgres.replication.status":
		return "Return streaming replication lag and status."
	case "postgres.replication.slots":
		return "List replication slots and their consumption status."
	case "postgres.wal.archiver":
		return "Check WAL archiving status and last archived WAL."
	case "postgres.storage.largest_relations":
		return "Find largest tables and indexes by on-disk size."
	default:
		return "PostgreSQL capability"
	}
}

func capabilityUseCases(name string) []string {
	switch name {
	case "postgres.connection.ping":
		return []string{"health_check", "connectivity_test", "troubleshooting"}
	case "postgres.connection.stats":
		return []string{"capacity_planning", "connection_saturation", "troubleshooting"}
	case "postgres.instance.info":
		return []string{"inventory", "version_check", "troubleshooting"}
	case "postgres.session.list":
		return []string{"performance_analysis", "blocking_query_detection", "troubleshooting"}
	case "postgres.query.top":
		return []string{"performance_analysis", "query_optimization", "troubleshooting"}
	case "postgres.table.stats":
		return []string{"capacity_planning", "index_optimization", "troubleshooting"}
	case "postgres.table.vacuum_status":
		return []string{"maintenance_planning", "bloat_detection", "troubleshooting"}
	case "postgres.lock.blocking":
		return []string{"deadlock_analysis", "query_blocking", "troubleshooting"}
	case "postgres.transaction.long_running":
		return []string{"performance_analysis", "lock_contention", "troubleshooting"}
	case "postgres.maintenance.progress":
		return []string{"maintenance_monitoring", "long_operation_tracking", "troubleshooting"}
	case "postgres.replication.status":
		return []string{"replica_health", "replication_lag", "troubleshooting"}
	case "postgres.replication.slots":
		return []string{"replication_monitoring", "slot_consumption", "troubleshooting"}
	case "postgres.wal.archiver":
		return []string{"backup_health", "archive_monitoring", "troubleshooting"}
	case "postgres.storage.largest_relations":
		return []string{"capacity_planning", "storage_bloat", "troubleshooting"}
	default:
		return nil
	}
}

func capabilityTags(name string) []string {
	switch name {
	case "postgres.connection.ping":
		return []string{"postgres", "connection", "health"}
	case "postgres.connection.stats":
		return []string{"postgres", "connection", "stats"}
	case "postgres.instance.info":
		return []string{"postgres", "instance", "metadata"}
	case "postgres.session.list":
		return []string{"postgres", "session", "activity"}
	case "postgres.query.top":
		return []string{"postgres", "query", "performance"}
	case "postgres.table.stats":
		return []string{"postgres", "table", "stats"}
	case "postgres.table.vacuum_status":
		return []string{"postgres", "table", "vacuum"}
	case "postgres.lock.blocking":
		return []string{"postgres", "lock", "blocking"}
	case "postgres.transaction.long_running":
		return []string{"postgres", "transaction", "performance"}
	case "postgres.maintenance.progress":
		return []string{"postgres", "maintenance", "progress"}
	case "postgres.replication.status":
		return []string{"postgres", "replication", "status"}
	case "postgres.replication.slots":
		return []string{"postgres", "replication", "slots"}
	case "postgres.wal.archiver":
		return []string{"postgres", "wal", "archiver"}
	case "postgres.storage.largest_relations":
		return []string{"postgres", "storage", "size"}
	default:
		return nil
	}
}

// ---------- Task stubs ----------
