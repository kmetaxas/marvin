package azure

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ provider.Provider = provider.BaseProvider{}
var _ provider.Provider = (*Provider)(nil)

func newTestProvider(clients *azureClients, cfgs ...config.AzureConfig) *Provider {
	cfg := config.AzureConfig{}
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	p := &Provider{}
	p.state = autoconfig.NewState(&p.mu, cfg, clients)
	return p
}

func TestNewProviderWiresAllCapabilities(t *testing.T) {
	t.Parallel()

	p := NewProvider(config.AzureConfig{})
	require.NotNil(t, p)
	assert.Equal(t, "azure", p.Name())

	capabilities := p.Capabilities()
	require.Len(t, capabilities, 9)

	names := map[string]bool{}
	for _, capability := range capabilities {
		names[capability.Name] = true
		assert.Equal(t, "azure", capability.Provider)
		assert.NotEmpty(t, capability.ParametersJSONSchema)
	}

	for _, name := range []string{
		"azure.network.nsg.list",
		"azure.network.udr.list",
		"azure.network.lb.rules.list",
		"azure.network.lb.rule.health",
		"azure.network.appgw.backend.health",
		"azure.network.appgw.listeners.list",
		"azure.logs.query_appgw",
		"azure.logs.query_vnet_flow",
		"azure.activity.changes",
	} {
		assert.True(t, names[name], name)
		_, ok := p.GetTask(name)
		assert.True(t, ok, name)
	}
}

func TestProviderUpdateConfigRecreatesClientsWhenChanged(t *testing.T) {
	t.Parallel()
	clientInitMu.Lock()
	defer clientInitMu.Unlock()

	origSecret := newClientSecretCredential
	origDefault := newDefaultAzureCredential
	newClientSecretCredential = func(tenantID, clientID, clientSecret string, options *azidentity.ClientSecretCredentialOptions) (azcore.TokenCredential, error) {
		return fakeCredential{}, nil
	}
	newDefaultAzureCredential = func(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return fakeCredential{}, nil
	}
	defer func() {
		newClientSecretCredential = origSecret
		newDefaultAzureCredential = origDefault
	}()

	p := NewProvider(config.AzureConfig{TenantID: "t1", ClientID: "c1", ClientSecret: "s1", SubscriptionID: "sub1"})
	require.NotNil(t, p)

	oldClients := p.state.Client()
	require.NotNil(t, oldClients)

	p.UpdateConfig("azure.network.nsg.list", map[string]any{
		"tenant_id":       "t2",
		"client_id":       "c2",
		"client_secret":   "s2",
		"subscription_id": "sub2",
		"resource_group":  "rg2",
		"workspace_id":    "ws2",
	})

	newCfg := p.currentConfig()
	assert.Equal(t, "t2", newCfg.TenantID)
	assert.Equal(t, "c2", newCfg.ClientID)
	assert.Equal(t, "s2", newCfg.ClientSecret)
	assert.Equal(t, "sub2", newCfg.SubscriptionID)
	assert.Equal(t, "rg2", newCfg.ResourceGroup)
	assert.Equal(t, "ws2", newCfg.WorkspaceID)
	assert.NotSame(t, oldClients, p.state.Client(), "clients should be recreated when config changes")
}

func TestProviderUpdateConfigDoesNotRecreateClientsWhenUnchanged(t *testing.T) {
	t.Parallel()
	clientInitMu.Lock()
	defer clientInitMu.Unlock()

	origSecret := newClientSecretCredential
	origDefault := newDefaultAzureCredential
	newClientSecretCredential = func(tenantID, clientID, clientSecret string, options *azidentity.ClientSecretCredentialOptions) (azcore.TokenCredential, error) {
		return fakeCredential{}, nil
	}
	newDefaultAzureCredential = func(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return fakeCredential{}, nil
	}
	defer func() {
		newClientSecretCredential = origSecret
		newDefaultAzureCredential = origDefault
	}()

	p := NewProvider(config.AzureConfig{TenantID: "t1", ClientID: "c1", ClientSecret: "s1", SubscriptionID: "sub1"})
	require.NotNil(t, p)

	oldClients := p.state.Client()
	require.NotNil(t, oldClients)

	p.UpdateConfig("azure.network.nsg.list", map[string]any{})

	assert.Same(t, oldClients, p.state.Client(), "clients should not be recreated when config is unchanged")
}
