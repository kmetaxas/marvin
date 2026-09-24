package provider

import (
	"context"
	"testing"

	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
	"github.com/stretchr/testify/assert"
)

func TestBaseProvider(t *testing.T) {
	t.Parallel()

	p := BaseProvider{
		ProviderName: "network",
		ProviderCapabilities: []capability.Capability{
			{Name: "network.dns.lookup", Provider: "network"},
		},
		EnabledFunc: func(config map[string]any) bool {
			return config["enabled"] == true
		},
	}

	assert.Equal(t, "network", p.Name())
	assert.Len(t, p.Capabilities(), 1)
	assert.True(t, p.IsEnabled(map[string]any{"enabled": true}))
	assert.False(t, p.IsEnabled(map[string]any{"enabled": false}))

	caps := p.Capabilities()
	caps[0].Name = "changed"
	assert.Equal(t, "network.dns.lookup", p.Capabilities()[0].Name)

	_, ok := p.GetTask("network.dns.lookup")
	assert.False(t, ok)
}

func TestBaseProviderDefaultEnabled(t *testing.T) {
	t.Parallel()

	p := BaseProvider{ProviderName: "os"}
	assert.True(t, p.IsEnabled(nil))
}

func TestBaseProviderGetTask(t *testing.T) {
	t.Parallel()

	mockTask := &mockTask{name: "network.dns.lookup"}
	p := BaseProvider{
		ProviderName: "network",
		Tasks: map[string]task.Task{
			"network.dns.lookup": mockTask,
		},
	}

	found, ok := p.GetTask("network.dns.lookup")
	assert.True(t, ok)
	assert.Equal(t, mockTask, found)

	_, ok = p.GetTask("network.missing")
	assert.False(t, ok)
}

func TestBaseProviderUpdateConfig(t *testing.T) {
	t.Parallel()
	p := BaseProvider{ProviderName: "os"}
	p.UpdateConfig("os.process.list", map[string]any{"key": "value"})
}

type mockTask struct{ name string }

func (m *mockTask) Name() string       { return m.name }
func (m *mockTask) JSONSchema() string { return `{"type":"object"}` }
func (m *mockTask) Execute(_ context.Context, _ map[string]any) (task.Result, error) {
	return task.Result{}, nil
}
