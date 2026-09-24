package prometheus

import (
	"sync"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

// Provider is the runtime-configurable Prometheus provider.
type Provider struct {
	mu    sync.RWMutex
	state *autoconfig.State[config.PrometheusConfig, prometheusClient]
	base  provider.BaseProvider
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

// IsEnabled always returns true for the Prometheus provider.
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

// CurrentClient returns the current Prometheus client under a read lock.
func (p *Provider) CurrentClient() prometheusClient {
	if p == nil || p.state == nil {
		return nil
	}
	return p.state.Client()
}

func (p *Provider) IsConfigured() bool {
	if p == nil || p.state == nil {
		return false
	}
	return p.state.Config().URL != ""
}

// UpdateConfig parses cfg fields and, if the configuration has changed,
// updates the stored config and recreates the Prometheus client.
func (p *Provider) UpdateConfig(capabilityName string, cfg map[string]any) {
	_ = capabilityName // not used; Prometheus uses a single shared config

	_, _, _ = autoconfig.Apply(p.state, cfg, autoconfig.Options[config.PrometheusConfig, prometheusClient]{
		Parse: parsePrometheusConfig,
		Build: func(cfg config.PrometheusConfig) (prometheusClient, error) {
			return newPrometheusClient(cfg), nil
		},
	})
}

func parsePrometheusConfig(_ config.PrometheusConfig, cfg map[string]any) (config.PrometheusConfig, error) {
	out := config.PrometheusConfig{}

	if u, ok := autoconfig.StringOK(cfg, "url"); ok {
		out.URL = u
	}
	if d, ok := autoconfig.DurationOK(cfg, "timeout"); ok {
		out.Timeout = d
	}

	out.Auth, _ = parsePrometheusAuthConfig(config.PrometheusAuthConfig{}, cfg)
	out.Guardrails, _ = parsePrometheusGuardrails(config.PrometheusGuardrails{}, cfg)

	return out, nil
}

func parsePrometheusAuthConfig(_ config.PrometheusAuthConfig, cfg map[string]any) (config.PrometheusAuthConfig, error) {
	out := config.PrometheusAuthConfig{}

	if v, ok := autoconfig.StringOK(cfg, "auth.type"); ok {
		out.Type = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.username"); ok {
		out.Username = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.password"); ok {
		out.Password = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.token"); ok {
		out.Token = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.header_name"); ok {
		out.HeaderName = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.header_value"); ok {
		out.HeaderValue = v
	}

	return out, nil
}

func parsePrometheusGuardrails(_ config.PrometheusGuardrails, cfg map[string]any) (config.PrometheusGuardrails, error) {
	out := config.PrometheusGuardrails{}

	if d, ok := autoconfig.DurationOK(cfg, "guardrails.max_query_range"); ok {
		out.MaxQueryRange = d
	}
	if d, ok := autoconfig.DurationOK(cfg, "guardrails.min_step"); ok {
		out.MinStep = d
	}
	if d, ok := autoconfig.DurationOK(cfg, "guardrails.query_timeout"); ok {
		out.QueryTimeout = d
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_returned_series"); ok {
		out.MaxReturnedSeries = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_returned_samples"); ok {
		out.MaxReturnedSamples = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_metadata_results"); ok {
		out.MaxMetadataResults = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_response_bytes"); ok {
		out.MaxResponseBytes = v
	}

	return out, nil
}

// NewProvider creates a new Prometheus provider with the given initial config.
func NewProvider(cfg config.PrometheusConfig) *Provider {
	client := prometheusClient(newPrometheusClient(cfg))
	p := &Provider{}
	p.state = autoconfig.NewState(&p.mu, cfg, client)

	tasks := []task.Task{
		&queryInstantTask{provider: nil},
		&queryRangeTask{provider: nil},
		&seriesListTask{provider: nil},
		&labelsListTask{provider: nil},
		&labelValuesTask{provider: nil},
		&metadataGetTask{provider: nil},
		&metricNamesTask{provider: nil},
		&alertsListTask{provider: nil},
		&rulesListTask{provider: nil},
		&targetsListTask{provider: nil},
		&targetGetTask{provider: nil},
		&targetMetadataTask{provider: nil},
		&scrapePoolsListTask{provider: nil},
		&exemplarsQueryTask{provider: nil},
		&statusRuntimeTask{provider: nil},
		&statusTsdbTask{provider: nil},
		&statusFlagsTask{provider: nil},
		&statusConfigTask{provider: nil},
		&statusReadyTask{provider: nil},
		&statusHealthyTask{provider: nil},
	}

	capabilities := make([]capability.Capability, 0, len(tasks))
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		capabilities = append(capabilities, capability.Capability{
			Name:                 t.Name(),
			Version:              "v1",
			Description:          capabilityDescription(t.Name()),
			Provider:             "prometheus",
			ParametersJSONSchema: t.JSONSchema(),
		})
		taskMap[t.Name()] = t
	}

	p.base = provider.BaseProvider{
		ProviderName:         "prometheus",
		ProviderCapabilities: capabilities,
		Tasks:                taskMap,
	}

	// Wire tasks back to the provider so they can call CurrentClient().
	for _, t := range tasks {
		switch task := t.(type) {
		case *queryInstantTask:
			task.provider = p
		case *queryRangeTask:
			task.provider = p
		case *seriesListTask:
			task.provider = p
		case *labelsListTask:
			task.provider = p
		case *labelValuesTask:
			task.provider = p
		case *metadataGetTask:
			task.provider = p
		case *metricNamesTask:
			task.provider = p
		case *alertsListTask:
			task.provider = p
		case *rulesListTask:
			task.provider = p
		case *targetsListTask:
			task.provider = p
		case *targetGetTask:
			task.provider = p
		case *targetMetadataTask:
			task.provider = p
		case *scrapePoolsListTask:
			task.provider = p
		case *exemplarsQueryTask:
			task.provider = p
		case *statusRuntimeTask:
			task.provider = p
		case *statusTsdbTask:
			task.provider = p
		case *statusFlagsTask:
			task.provider = p
		case *statusConfigTask:
			task.provider = p
		case *statusReadyTask:
			task.provider = p
		case *statusHealthyTask:
			task.provider = p
		}
	}

	return p
}

func capabilityDescription(name string) string {
	switch name {
	case "prometheus.query.instant":
		return "Executes an instant PromQL query at a point in time"
	case "prometheus.query.range":
		return "Executes a PromQL range query over a time interval"
	case "prometheus.series.list":
		return "Finds time series matching label selectors"
	case "prometheus.labels.list":
		return "Lists known label names"
	case "prometheus.label.values":
		return "Lists values for a specific label name"
	case "prometheus.metadata.get":
		return "Returns metric metadata (type, help, unit)"
	case "prometheus.metric.names":
		return "Lists available metric names"
	case "prometheus.alerts.list":
		return "Lists active and pending alerts"
	case "prometheus.rules.list":
		return "Lists alerting and recording rules"
	case "prometheus.targets.list":
		return "Lists scrape targets and their health"
	case "prometheus.target.get":
		return "Returns detailed information for a single target"
	case "prometheus.target.metadata":
		return "Returns metadata about metrics scraped from targets"
	case "prometheus.scrape_pools.list":
		return "Lists scrape pools with target counts"
	case "prometheus.exemplars.query":
		return "Queries exemplars for a PromQL expression"
	case "prometheus.status.runtime":
		return "Returns Prometheus runtime and build information"
	case "prometheus.status.tsdb":
		return "Returns Prometheus TSDB statistics and cardinality"
	case "prometheus.status.flags":
		return "Returns Prometheus command-line flags"
	case "prometheus.status.config":
		return "Returns the loaded Prometheus configuration (redacted)"
	case "prometheus.status.ready":
		return "Checks whether Prometheus is ready to serve queries"
	case "prometheus.status.healthy":
		return "Checks Prometheus basic health"
	default:
		return "Prometheus capability"
	}
}
