package registry

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
	marvinpb "github.com/marvin-agent/marvin/pkg/proto/marvin"
)

// OnCapabilitiesChanged is called when the set of enabled capabilities changes.
type OnCapabilitiesChanged func(caps []capability.Capability)

type Registry struct {
	mu              sync.RWMutex
	providers       map[string]provider.Provider
	capabilities    map[string]capability.Capability
	tasks           map[string]task.Task
	enabledPatterns []string
	onChanged       OnCapabilitiesChanged
}

func New() *Registry {
	return &Registry{
		providers:    make(map[string]provider.Provider),
		capabilities: make(map[string]capability.Capability),
		tasks:        make(map[string]task.Task),
	}
}

func (r *Registry) SetOnCapabilitiesChanged(fn OnCapabilitiesChanged) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onChanged = fn
}

func (r *Registry) RegisterProvider(p provider.Provider, cfg map[string]any) error {
	if p == nil {
		return fmt.Errorf("provider is nil")
	}
	if !p.IsEnabled(cfg) {
		return nil
	}

	enabledPatterns, err := parseEnabledFilter(cfg)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if enabledPatterns != nil {
		r.enabledPatterns = enabledPatterns
	}

	// Always store the provider so runtime config updates can find it.
	if _, exists := r.providers[p.Name()]; !exists {
		r.providers[p.Name()] = p
	}

	for _, cap := range p.Capabilities() {
		if err := cap.Validate(); err != nil {
			return fmt.Errorf("provider %q capability %q: %w", p.Name(), cap.Name, err)
		}
		if _, exists := r.capabilities[cap.Name]; exists {
			return fmt.Errorf("duplicate capability %q", cap.Name)
		}
		if cap.Provider == "" {
			cap.Provider = p.Name()
		}
		r.capabilities[cap.Name] = cap

		if t, ok := p.GetTask(cap.Name); ok {
			r.tasks[cap.Name] = t
		}
	}

	r.emitChangedLocked()
	return nil
}

func (r *Registry) IsEnabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.capabilities[name]
	return ok && r.isEnabledLocked(name)
}

func (r *Registry) ListEnabled() []capability.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []capability.Capability
	for name, cap := range r.capabilities {
		if r.isEnabledLocked(name) {
			cap.Enabled = true
			out = append(out, cap)
		}
	}
	sortCapabilities(out)
	return out
}

func (r *Registry) ListAll() []capability.Capability {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]capability.Capability, 0, len(r.capabilities))
	for name, cap := range r.capabilities {
		cap.Enabled = r.isEnabledLocked(name)
		out = append(out, cap)
	}
	sortCapabilities(out)
	return out
}

func (r *Registry) Get(name string) (capability.Capability, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cap, ok := r.capabilities[name]
	if ok {
		cap.Enabled = r.isEnabledLocked(name)
	}
	return cap, ok
}

func (r *Registry) GetTask(name string) (task.Task, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tasks[name]
	return t, ok
}

func (r *Registry) isEnabledLocked(name string) bool {
	if len(r.enabledPatterns) == 0 {
		return true
	}
	for _, pattern := range r.enabledPatterns {
		if capability.IsPattern(pattern) {
			matched, err := capability.Match(name, pattern)
			if err == nil && matched {
				return true
			}
			continue
		}
		if pattern == name {
			return true
		}
	}
	return false
}

func parseEnabledFilter(cfg map[string]any) ([]string, error) {
	if len(cfg) == 0 {
		return nil, nil
	}
	raw, ok := cfg["enabled"]
	if !ok || raw == nil {
		return nil, nil
	}

	var result []string
	switch values := raw.(type) {
	case []string:
		result = append(result, values...)
	case []any:
		for _, value := range values {
			name, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("capabilities.enabled entries must be strings")
			}
			result = append(result, name)
		}
	default:
		return nil, fmt.Errorf("capabilities.enabled must be a string list")
	}

	return result, nil
}

func (r *Registry) ApplyOnlineConfig(upd *marvinpb.UpdateConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	changed := false
	for _, manifest := range upd.GetCapabilities() {
		name := manifest.GetName()
		cap, ok := r.capabilities[name]
		if !ok {
			// Not a known capability, but could be a new provider-level config.
			// Find the provider by capability prefix (e.g. "prometheus.query.instant" -> "prometheus")
			if providerName := findProviderByCapability(r.providers, name); providerName != "" {
				provider, pok := r.providers[providerName]
				if pok {
					cfg := manifest.GetConfig()
					if cfg != nil {
						provider.UpdateConfig(name, cfg.AsMap())
					}
				}
			}
			continue
		}
		providerName := cap.Provider
		if providerName == "" {
			continue
		}
		provider, ok := r.providers[providerName]
		if !ok {
			continue
		}
		cfg := manifest.GetConfig()
		if cfg == nil {
			continue
		}
		provider.UpdateConfig(name, cfg.AsMap())
	}

	// After updating configs, re-evaluate each provider's configured state.
	for _, p := range r.providers {
		wasConfigured := r.hasAnyCapabilityLocked(p.Name())
		isConfigured := p.IsConfigured()

		if wasConfigured == isConfigured {
			continue
		}

		if isConfigured {
			// Provider became configured: add its capabilities.
			for _, cap := range p.Capabilities() {
				if cap.Provider == "" {
					cap.Provider = p.Name()
				}
				r.capabilities[cap.Name] = cap
				if t, ok := p.GetTask(cap.Name); ok {
					r.tasks[cap.Name] = t
				}
			}
		} else {
			// Provider became unconfigured: remove its capabilities.
			for _, cap := range p.Capabilities() {
				delete(r.capabilities, cap.Name)
				delete(r.tasks, cap.Name)
			}
		}
		changed = true
	}

	if changed {
		r.emitChangedLocked()
	}
}

func findProviderByCapability(providers map[string]provider.Provider, capabilityName string) string {
	if _, ok := providers[capabilityName]; ok {
		return capabilityName
	}
	for providerName := range providers {
		if providerName != "" && strings.HasPrefix(capabilityName, providerName+".") {
			return providerName
		}
	}
	return ""
}

func (r *Registry) hasAnyCapabilityLocked(providerName string) bool {
	for _, cap := range r.capabilities {
		if cap.Provider == providerName {
			return true
		}
	}
	return false
}

func (r *Registry) emitChangedLocked() {
	if r.onChanged == nil {
		return
	}
	var caps []capability.Capability
	for name, cap := range r.capabilities {
		if r.isEnabledLocked(name) {
			cap.Enabled = true
			caps = append(caps, cap)
		}
	}
	sortCapabilities(caps)
	// Release lock before calling callback to avoid deadlock.
	r.mu.Unlock()
	r.onChanged(caps)
	r.mu.Lock()
}

func sortCapabilities(caps []capability.Capability) {
	sort.Slice(caps, func(i, j int) bool {
		return caps[i].Name < caps[j].Name
	})
}
