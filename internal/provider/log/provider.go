package log

import (
	"sync"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

// Provider is the runtime-configurable Graylog provider.
type Provider struct {
	mu    sync.RWMutex
	state *autoconfig.State[config.GraylogConfig, graylogClient]
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

// IsEnabled always returns true for the Graylog provider.
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

// CurrentClient returns the current Graylog client under a read lock.
func (p *Provider) CurrentClient() graylogClient {
	return p.state.Client()
}

// IsConfigured reports whether the provider has a non-empty URL.
func (p *Provider) IsConfigured() bool {
	return p.state.Config().URL != ""
}

// UpdateConfig parses cfg fields and, if the configuration has changed,
// updates the stored config and recreates the Graylog client.
func (p *Provider) UpdateConfig(capabilityName string, cfg map[string]any) {
	_ = capabilityName
	_, _, _ = autoconfig.Apply(p.state, cfg, autoconfig.Options[config.GraylogConfig, graylogClient]{
		Parse: parseGraylogConfig,
		Build: func(cfg config.GraylogConfig) (graylogClient, error) {
			return newGraylogClient(cfg), nil
		},
	})
}

func parseGraylogConfig(_ config.GraylogConfig, cfg map[string]any) (config.GraylogConfig, error) {
	out := config.GraylogConfig{}

	if u, ok := autoconfig.StringOK(cfg, "url"); ok {
		out.URL = u
	}
	if d, ok := autoconfig.DurationOK(cfg, "timeout"); ok {
		out.Timeout = d
	}

	out.Auth, _ = parseGraylogAuthConfig(config.GraylogAuthConfig{}, cfg)
	out.Guardrails, _ = parseGraylogGuardrails(config.GraylogGuardrails{}, cfg)

	return out, nil
}

func parseGraylogAuthConfig(_ config.GraylogAuthConfig, cfg map[string]any) (config.GraylogAuthConfig, error) {
	out := config.GraylogAuthConfig{}

	if v, ok := autoconfig.StringOK(cfg, "auth.type"); ok {
		out.Type = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.token"); ok {
		out.Token = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.username"); ok {
		out.Username = v
	}
	if v, ok := autoconfig.StringOK(cfg, "auth.password"); ok {
		out.Password = v
	}

	return out, nil
}

func parseGraylogGuardrails(_ config.GraylogGuardrails, cfg map[string]any) (config.GraylogGuardrails, error) {
	out := config.GraylogGuardrails{}

	if d, ok := autoconfig.DurationOK(cfg, "guardrails.max_query_range"); ok {
		out.MaxQueryRange = d
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_results"); ok {
		out.MaxResults = v
	}
	if v, ok := autoconfig.IntOK(cfg, "guardrails.max_histogram_buckets"); ok {
		out.MaxHistogramBuckets = v
	}
	if d, ok := autoconfig.DurationOK(cfg, "guardrails.query_timeout"); ok {
		out.QueryTimeout = d
	}

	return out, nil
}

// NewProvider creates a new Graylog provider with the given initial config.
func NewProvider(cfg config.GraylogConfig) *Provider {
	client := newGraylogClient(cfg)
	p := &Provider{}
	p.state = autoconfig.NewState(&p.mu, cfg, client)

	tasks := []task.Task{
		&searchTask{provider: nil},
		&countTask{provider: nil},
		&histogramTask{provider: nil},
		&fieldstatsTask{provider: nil},
	}

	capabilities := make([]capability.Capability, 0, len(tasks))
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		capabilities = append(capabilities, capability.Capability{
			Name:                 t.Name(),
			Version:              "v1",
			Description:          capabilityDescription(t.Name()),
			Provider:             "log",
			ParametersJSONSchema: t.JSONSchema(),
		})
		taskMap[t.Name()] = t
	}

	p.base = provider.BaseProvider{
		ProviderName:         "log",
		ProviderCapabilities: capabilities,
		Tasks:                taskMap,
	}

	// Wire tasks back to the provider so they can call CurrentClient().
	for _, t := range tasks {
		switch task := t.(type) {
		case *searchTask:
			task.provider = p
		case *countTask:
			task.provider = p
		case *histogramTask:
			task.provider = p
		case *fieldstatsTask:
			task.provider = p
		}
	}

	return p
}

func capabilityDescription(name string) string {
	switch name {
	case "log.graylog.search":
		return "Search Graylog messages with filters, time range, and field selection"
	case "log.graylog.count":
		return "Return the number of matching log messages without retrieving them"
	case "log.graylog.histogram":
		return "Return time-bucketed hit counts for a query"
	case "log.graylog.fieldstats":
		return "Return descriptive statistics (min/max/avg/sum) for a numeric field"
	default:
		return "Graylog capability"
	}
}
