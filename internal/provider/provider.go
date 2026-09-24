package provider

import (
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

type Provider interface {
	Name() string
	Capabilities() []capability.Capability
	IsEnabled(config map[string]any) bool
	GetTask(name string) (task.Task, bool)
	UpdateConfig(capabilityName string, cfg map[string]any)
	IsConfigured() bool
}

type BaseProvider struct {
	ProviderName         string
	ProviderCapabilities []capability.Capability
	EnabledFunc          func(config map[string]any) bool
	Tasks                map[string]task.Task
}

func (p BaseProvider) Name() string {
	return p.ProviderName
}

func (p BaseProvider) Capabilities() []capability.Capability {
	out := make([]capability.Capability, len(p.ProviderCapabilities))
	copy(out, p.ProviderCapabilities)
	return out
}

func (p BaseProvider) IsEnabled(config map[string]any) bool {
	if p.EnabledFunc != nil {
		return p.EnabledFunc(config)
	}

	return true
}

func (p BaseProvider) GetTask(name string) (task.Task, bool) {
	if p.Tasks == nil {
		return nil, false
	}
	t, ok := p.Tasks[name]
	return t, ok
}

func (p BaseProvider) UpdateConfig(capabilityName string, cfg map[string]any) {}

func (p BaseProvider) IsConfigured() bool { return true }
