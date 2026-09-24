package azure

import (
	"sync"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/autoconfig"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

type Provider struct {
	mu    sync.RWMutex
	state *autoconfig.State[config.AzureConfig, *azureClients]
	provider.BaseProvider
}

func NewProvider(cfg config.AzureConfig) *Provider {
	p := &Provider{}
	p.state = autoconfig.NewState(&p.mu, cfg, newAzureClients(cfg))

	nsgTask := &nsgTask{provider: p}
	udrTask := &udrTask{provider: p}
	lbRulesTask := &lbRulesTask{provider: p}
	lbHealthTask := &lbHealthTask{provider: p}
	appgwBackendTask := &appGatewayBackendHealthTask{provider: p}
	appgwListenersTask := &appGatewayListenersTask{provider: p}
	logsAppGWTask := &appGatewayLogsTask{provider: p}
	logsVNetTask := &vnetFlowLogsTask{provider: p}
	activityTask := &activityChangesTask{provider: p}

	tasks := []task.Task{nsgTask, udrTask, lbRulesTask, lbHealthTask, appgwBackendTask, appgwListenersTask, logsAppGWTask, logsVNetTask, activityTask}
	capabilities := make([]capability.Capability, 0, len(tasks))
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		capabilities = append(capabilities, capability.Capability{
			Name:                 t.Name(),
			Version:              "v1",
			Description:          capabilityDescription(t.Name()),
			Provider:             "azure",
			ParametersJSONSchema: t.JSONSchema(),
		})
		taskMap[t.Name()] = t
	}

	p.BaseProvider = provider.BaseProvider{
		ProviderName:         "azure",
		ProviderCapabilities: capabilities,
		Tasks:                taskMap,
	}

	return p
}

func (p *Provider) UpdateConfig(capabilityName string, cfg map[string]any) {
	_ = capabilityName
	_, _, _ = autoconfig.Apply(p.state, cfg, autoconfig.Options[config.AzureConfig, *azureClients]{
		Parse: parseAzureConfig,
		Equal: func(a, b config.AzureConfig) bool {
			return a == b
		},
		Build: func(cfg config.AzureConfig) (*azureClients, error) {
			return newAzureClients(cfg), nil
		},
	})
}

func (p *Provider) IsConfigured() bool {
	cfg := p.state.Config()
	return cfg.TenantID != "" && cfg.ClientID != "" && cfg.ClientSecret != "" && cfg.SubscriptionID != ""
}

func (p *Provider) currentConfig() config.AzureConfig {
	return p.state.Config()
}

func (p *Provider) CurrentNSGClient() securityGroupClient {
	return p.state.Client().nsg
}

func (p *Provider) CurrentUDRClient() routeTableClient {
	return p.state.Client().udr
}

func (p *Provider) CurrentLBClient() loadBalancerClient {
	return p.state.Client().lb
}

func (p *Provider) CurrentLBProbeClient() loadBalancerProbeClient {
	return p.state.Client().lbProbes
}

func (p *Provider) CurrentAppGatewayClient() applicationGatewayClient {
	return p.state.Client().appGateway
}

func (p *Provider) CurrentLogsClient() logsClient {
	return p.state.Client().logs
}

func (p *Provider) CurrentActivityClient() activityLogsClient {
	return p.state.Client().activity
}

func parseAzureConfig(base config.AzureConfig, cfg map[string]any) (config.AzureConfig, error) {
	if v, ok := autoconfig.StringOK(cfg, "tenant_id"); ok {
		base.TenantID = v
	}
	if v, ok := autoconfig.StringOK(cfg, "client_id"); ok {
		base.ClientID = v
	}
	if v, ok := autoconfig.StringOK(cfg, "client_secret"); ok {
		base.ClientSecret = v
	}
	if v, ok := autoconfig.StringOK(cfg, "subscription_id"); ok {
		base.SubscriptionID = v
	}
	if v, ok := autoconfig.StringOK(cfg, "resource_group"); ok {
		base.ResourceGroup = v
	}
	if v, ok := autoconfig.StringOK(cfg, "workspace_id"); ok {
		base.WorkspaceID = v
	}

	return base, nil
}

func capabilityDescription(name string) string {
	switch name {
	case "azure.network.nsg.list":
		return "Lists Azure network security groups and optional rules"
	case "azure.network.udr.list":
		return "Lists Azure user-defined routes and optional route entries"
	case "azure.network.lb.rules.list":
		return "Lists Azure load balancer rules"
	case "azure.network.lb.rule.health":
		return "Lists Azure load balancer probe health configuration"
	case "azure.network.appgw.backend.health":
		return "Returns Azure application gateway backend health"
	case "azure.network.appgw.listeners.list":
		return "Lists Azure application gateway listeners"
	case "azure.logs.query_appgw":
		return "Queries Azure Application Gateway logs"
	case "azure.logs.query_vnet_flow":
		return "Queries Azure VNet flow logs"
	case "azure.activity.changes":
		return "Lists Azure activity log changes"
	default:
		return "Azure capability"
	}
}
