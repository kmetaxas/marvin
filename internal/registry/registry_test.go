package registry

import (
	"context"
	"testing"

	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
	marvinpb "github.com/marvin-agent/marvin/pkg/proto/marvin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestRegisterProviderAndLists(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName: "network",
		ProviderCapabilities: []capability.Capability{
			{Name: "network.dns.lookup"},
			{Name: "network.icmp.echo_request"},
		},
	}, map[string]any{"enabled": []string{"network.dns.lookup"}}))

	assert.Equal(t, []capability.Capability{
		{Name: "network.dns.lookup", Provider: "network", Enabled: true},
		{Name: "network.icmp.echo_request", Provider: "network", Enabled: false},
	}, r.ListAll())
	assert.Equal(t, []capability.Capability{{Name: "network.dns.lookup", Provider: "network", Enabled: true}}, r.ListEnabled())
	assert.True(t, r.IsEnabled("network.dns.lookup"))
	assert.False(t, r.IsEnabled("network.icmp.echo_request"))
}

func TestRegisterProviderDuplicateCapability(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName:         "network",
		ProviderCapabilities: []capability.Capability{{Name: "network.dns.lookup"}},
	}, nil))

	err := r.RegisterProvider(provider.BaseProvider{
		ProviderName:         "dns",
		ProviderCapabilities: []capability.Capability{{Name: "network.dns.lookup"}},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate capability")
}

func TestRegisterProviderDisabled(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName:         "os",
		ProviderCapabilities: []capability.Capability{{Name: "os.process.list"}},
		EnabledFunc: func(cfg map[string]any) bool {
			return false
		},
	}, nil))

	assert.Empty(t, r.ListAll())
}

func TestGet(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName:         "os",
		ProviderCapabilities: []capability.Capability{{Name: "os.process.list"}},
	}, nil))

	cap, ok := r.Get("os.process.list")
	require.True(t, ok)
	assert.Equal(t, capability.Capability{Name: "os.process.list", Provider: "os", Enabled: true}, cap)

	_, ok = r.Get("missing")
	assert.False(t, ok)
}

func TestGetTask(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName:         "os",
		ProviderCapabilities: []capability.Capability{{Name: "os.process.list"}},
		Tasks:                map[string]task.Task{"os.process.list": mockTask{name: "os.process.list"}},
	}, nil))

	tk, ok := r.GetTask("os.process.list")
	require.True(t, ok)
	assert.Equal(t, "os.process.list", tk.Name())

	_, ok = r.GetTask("missing")
	assert.False(t, ok)
}

func TestRegisterProviderWithPatterns(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName: "kubernetes",
		ProviderCapabilities: []capability.Capability{
			{Name: "kubernetes.pod.list"},
			{Name: "kubernetes.pod.get"},
			{Name: "kubernetes.service.list"},
		},
	}, map[string]any{"enabled": []string{"kubernetes.pod.*"}}))

	assert.True(t, r.IsEnabled("kubernetes.pod.list"))
	assert.True(t, r.IsEnabled("kubernetes.pod.get"))
	assert.False(t, r.IsEnabled("kubernetes.service.list"))

	assert.Len(t, r.ListEnabled(), 2)
	assert.Len(t, r.ListAll(), 3)
}

func TestRegisterProviderWithGlobalWildcard(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName: "network",
		ProviderCapabilities: []capability.Capability{
			{Name: "network.dns.lookup"},
			{Name: "network.icmp.echo_request"},
		},
	}, map[string]any{"enabled": []string{"*"}}))

	assert.True(t, r.IsEnabled("network.dns.lookup"))
	assert.True(t, r.IsEnabled("network.icmp.echo_request"))
	assert.Len(t, r.ListEnabled(), 2)
}

func TestRegisterProviderMixedExactAndPattern(t *testing.T) {
	t.Parallel()

	r := New()
	require.NoError(t, r.RegisterProvider(provider.BaseProvider{
		ProviderName: "kubernetes",
		ProviderCapabilities: []capability.Capability{
			{Name: "kubernetes.pod.list"},
			{Name: "kubernetes.pod.get"},
			{Name: "kubernetes.service.list"},
			{Name: "kubernetes.deployment.list"},
		},
	}, map[string]any{"enabled": []string{"kubernetes.pod.*", "kubernetes.deployment.list"}}))

	assert.True(t, r.IsEnabled("kubernetes.pod.list"))
	assert.True(t, r.IsEnabled("kubernetes.pod.get"))
	assert.False(t, r.IsEnabled("kubernetes.service.list"))
	assert.True(t, r.IsEnabled("kubernetes.deployment.list"))
	assert.Len(t, r.ListEnabled(), 3)
}

func TestApplyOnlineConfig(t *testing.T) {
	t.Parallel()

	mockProv := &mockProviderWithUpdateConfig{
		BaseProvider: provider.BaseProvider{
			ProviderName: "test",
			ProviderCapabilities: []capability.Capability{
				{Name: "test.cap.one"},
				{Name: "test.cap.two"},
			},
		},
	}

	r := New()
	require.NoError(t, r.RegisterProvider(mockProv, nil))

	cfg, err := structpb.NewStruct(map[string]any{"url": "https://new"})
	require.NoError(t, err)

	r.ApplyOnlineConfig(&marvinpb.UpdateConfig{
		Capabilities: []*marvinpb.CapabilityManifest{
			{Name: "test.cap.one", Config: cfg},
			{Name: "unknown.cap", Config: cfg},
		},
	})

	assert.Len(t, mockProv.updated, 1)
	assert.Equal(t, "test.cap.one", mockProv.updated[0].name)
	assert.Equal(t, map[string]any{"url": "https://new"}, mockProv.updated[0].cfg)
}

func TestApplyOnlineConfigActivatesUnconfiguredProvider(t *testing.T) {
	t.Parallel()

	mockProv := &mockProviderWithUpdateConfig{
		BaseProvider: provider.BaseProvider{
			ProviderName: "prometheus",
			ProviderCapabilities: []capability.Capability{
				{Name: "prometheus.query.instant"},
				{Name: "prometheus.query.range"},
			},
		},
	}

	r := New()
	require.NoError(t, r.RegisterProvider(mockProv, nil))

	assert.Len(t, r.ListAll(), 2)
	assert.True(t, r.IsEnabled("prometheus.query.instant"))

	cfg, err := structpb.NewStruct(map[string]any{"url": "http://prometheus:9090"})
	require.NoError(t, err)

	r.ApplyOnlineConfig(&marvinpb.UpdateConfig{
		Capabilities: []*marvinpb.CapabilityManifest{
			{Name: "prometheus", Config: cfg},
		},
	})

	require.Len(t, mockProv.updated, 1)
	assert.Equal(t, "prometheus", mockProv.updated[0].name)
	assert.Equal(t, map[string]any{"url": "http://prometheus:9090"}, mockProv.updated[0].cfg)
}

func TestApplyOnlineConfigActivatesUnconfiguredProviderByPrefix(t *testing.T) {
	t.Parallel()

	mockProv := &mockProviderWithUpdateConfig{
		BaseProvider: provider.BaseProvider{
			ProviderName: "prometheus",
			ProviderCapabilities: []capability.Capability{
				{Name: "prometheus.query.instant"},
			},
		},
	}

	r := New()
	require.NoError(t, r.RegisterProvider(mockProv, nil))

	cfg, err := structpb.NewStruct(map[string]any{"url": "http://prometheus:9090"})
	require.NoError(t, err)

	r.ApplyOnlineConfig(&marvinpb.UpdateConfig{
		Capabilities: []*marvinpb.CapabilityManifest{
			{Name: "prometheus.query.instant", Config: cfg},
		},
	})

	require.Len(t, mockProv.updated, 1)
	assert.Equal(t, "prometheus.query.instant", mockProv.updated[0].name)
	assert.Equal(t, map[string]any{"url": "http://prometheus:9090"}, mockProv.updated[0].cfg)
}

type mockProviderWithUpdateConfig struct {
	provider.BaseProvider
	updated []struct {
		name string
		cfg  map[string]any
	}
}

func (m *mockProviderWithUpdateConfig) UpdateConfig(capabilityName string, cfg map[string]any) {
	m.updated = append(m.updated, struct {
		name string
		cfg  map[string]any
	}{name: capabilityName, cfg: cfg})
}

type mockTask struct{ name string }

func (m mockTask) Name() string       { return m.name }
func (m mockTask) JSONSchema() string { return `{"type":"object"}` }
func (m mockTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	return task.Result{}, nil
}
