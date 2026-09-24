package azure

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider/common"
)

var (
	newClientSecretCredential = func(tenantID, clientID, clientSecret string, options *azidentity.ClientSecretCredentialOptions) (azcore.TokenCredential, error) {
		return azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, options)
	}
	newDefaultAzureCredential = func(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		return azidentity.NewDefaultAzureCredential(options)
	}
	newSecurityGroupsClient = func(subscriptionID string, cred azcore.TokenCredential) (*armnetwork.SecurityGroupsClient, error) {
		return armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	}
	newRouteTablesClient = func(subscriptionID string, cred azcore.TokenCredential) (*armnetwork.RouteTablesClient, error) {
		return armnetwork.NewRouteTablesClient(subscriptionID, cred, nil)
	}
	newLoadBalancersClient = func(subscriptionID string, cred azcore.TokenCredential) (*armnetwork.LoadBalancersClient, error) {
		return armnetwork.NewLoadBalancersClient(subscriptionID, cred, nil)
	}
	newLoadBalancerProbesClient = func(subscriptionID string, cred azcore.TokenCredential) (*armnetwork.LoadBalancerProbesClient, error) {
		return armnetwork.NewLoadBalancerProbesClient(subscriptionID, cred, nil)
	}
	newApplicationGatewaysClient = func(subscriptionID string, cred azcore.TokenCredential) (*armnetwork.ApplicationGatewaysClient, error) {
		return armnetwork.NewApplicationGatewaysClient(subscriptionID, cred, nil)
	}
	newActivityLogsClient = func(subscriptionID string, cred azcore.TokenCredential) (*armmonitor.ActivityLogsClient, error) {
		return armmonitor.NewActivityLogsClient(subscriptionID, cred, nil)
	}
	newLogsClient = func(cred azcore.TokenCredential) (*azquery.LogsClient, error) {
		return azquery.NewLogsClient(cred, nil)
	}
)

type securityGroupRule struct {
	Name                     string `json:"name,omitempty"`
	Description              string `json:"description,omitempty"`
	Direction                string `json:"direction,omitempty"`
	Access                   string `json:"access,omitempty"`
	Protocol                 string `json:"protocol,omitempty"`
	Priority                 int32  `json:"priority,omitempty"`
	SourcePortRange          string `json:"source_port_range,omitempty"`
	DestinationPortRange     string `json:"destination_port_range,omitempty"`
	SourceAddressPrefix      string `json:"source_address_prefix,omitempty"`
	DestinationAddressPrefix string `json:"destination_address_prefix,omitempty"`
}

type securityGroupInfo struct {
	ID            string              `json:"id,omitempty"`
	Name          string              `json:"name,omitempty"`
	Location      string              `json:"location,omitempty"`
	ResourceGroup string              `json:"resource_group,omitempty"`
	Rules         []securityGroupRule `json:"rules,omitempty"`
}

type routeInfo struct {
	Name          string `json:"name,omitempty"`
	AddressPrefix string `json:"address_prefix,omitempty"`
	NextHopType   string `json:"next_hop_type,omitempty"`
	NextHopIP     string `json:"next_hop_ip,omitempty"`
}

type routeTableInfo struct {
	ID            string      `json:"id,omitempty"`
	Name          string      `json:"name,omitempty"`
	Location      string      `json:"location,omitempty"`
	ResourceGroup string      `json:"resource_group,omitempty"`
	Routes        []routeInfo `json:"routes,omitempty"`
}

type loadBalancerRuleInfo struct {
	ID                  string `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	Protocol            string `json:"protocol,omitempty"`
	FrontendPort        int32  `json:"frontend_port,omitempty"`
	BackendPort         int32  `json:"backend_port,omitempty"`
	FrontendIPConfigID  string `json:"frontend_ip_configuration_id,omitempty"`
	BackendPoolID       string `json:"backend_address_pool_id,omitempty"`
	ProbeID             string `json:"probe_id,omitempty"`
	IdleTimeoutMinutes  int32  `json:"idle_timeout_minutes,omitempty"`
	EnableFloatingIP    bool   `json:"enable_floating_ip,omitempty"`
	DisableOutboundSNAT bool   `json:"disable_outbound_snat,omitempty"`
}

type loadBalancerProbeInfo struct {
	ID                 string `json:"id,omitempty"`
	Name               string `json:"name,omitempty"`
	Protocol           string `json:"protocol,omitempty"`
	Port               int32  `json:"port,omitempty"`
	RequestPath        string `json:"request_path,omitempty"`
	IntervalSeconds    int32  `json:"interval_seconds,omitempty"`
	UnhealthyThreshold int32  `json:"unhealthy_threshold,omitempty"`
}

type appGatewayListenerInfo struct {
	ID                          string   `json:"id,omitempty"`
	Name                        string   `json:"name,omitempty"`
	Protocol                    string   `json:"protocol,omitempty"`
	HostName                    string   `json:"host_name,omitempty"`
	HostNames                   []string `json:"host_names,omitempty"`
	FrontendPortID              string   `json:"frontend_port_id,omitempty"`
	FrontendIPConfigID          string   `json:"frontend_ip_configuration_id,omitempty"`
	RequireServerNameIndication bool     `json:"require_server_name_indication,omitempty"`
}

type appGatewayBackendServerInfo struct {
	Address string `json:"address,omitempty"`
	Health  string `json:"health,omitempty"`
}

type appGatewayBackendHTTPSettingsInfo struct {
	Name    string                        `json:"name,omitempty"`
	ID      string                        `json:"id,omitempty"`
	Servers []appGatewayBackendServerInfo `json:"servers,omitempty"`
}

type appGatewayBackendPoolInfo struct {
	Name         string                              `json:"name,omitempty"`
	ID           string                              `json:"id,omitempty"`
	HTTPSettings []appGatewayBackendHTTPSettingsInfo `json:"http_settings,omitempty"`
}

type logQueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type logQueryTable struct {
	Name    string           `json:"name,omitempty"`
	Columns []logQueryColumn `json:"columns,omitempty"`
	Rows    []map[string]any `json:"rows,omitempty"`
}

type activityEventInfo struct {
	EventTimestamp string `json:"event_timestamp,omitempty"`
	OperationName  string `json:"operation_name,omitempty"`
	Status         string `json:"status,omitempty"`
	ResourceGroup  string `json:"resource_group,omitempty"`
	ResourceType   string `json:"resource_type,omitempty"`
	ResourceID     string `json:"resource_id,omitempty"`
	Caller         string `json:"caller,omitempty"`
	Category       string `json:"category,omitempty"`
	CorrelationID  string `json:"correlation_id,omitempty"`
	Level          string `json:"level,omitempty"`
	SubStatus      string `json:"sub_status,omitempty"`
	Description    string `json:"description,omitempty"`
}

type securityGroupClient interface {
	ListAll(ctx context.Context) ([]securityGroupInfo, error)
	ListByResourceGroup(ctx context.Context, resourceGroup string) ([]securityGroupInfo, error)
}

type routeTableClient interface {
	ListAll(ctx context.Context) ([]routeTableInfo, error)
	ListByResourceGroup(ctx context.Context, resourceGroup string) ([]routeTableInfo, error)
}

type loadBalancerClient interface {
	GetRules(ctx context.Context, resourceGroup, loadBalancerName string) ([]loadBalancerRuleInfo, error)
}

type loadBalancerProbeClient interface {
	List(ctx context.Context, resourceGroup, loadBalancerName string) ([]loadBalancerProbeInfo, error)
}

type applicationGatewayClient interface {
	BackendHealth(ctx context.Context, resourceGroup, appGatewayName string) ([]appGatewayBackendPoolInfo, error)
	GetListeners(ctx context.Context, resourceGroup, appGatewayName string) ([]appGatewayListenerInfo, error)
}

type logsClient interface {
	QueryWorkspace(ctx context.Context, workspaceID, query string, start, end time.Time) ([]logQueryTable, error)
}

type activityLogsClient interface {
	List(ctx context.Context, filter string) ([]activityEventInfo, error)
}

type azureClients struct {
	cfg        config.AzureConfig
	cred       azcore.TokenCredential
	initErr    error
	nsg        securityGroupClient
	udr        routeTableClient
	lb         loadBalancerClient
	lbProbes   loadBalancerProbeClient
	appGateway applicationGatewayClient
	logs       logsClient
	activity   activityLogsClient
}

func newAzureClients(cfg config.AzureConfig) *azureClients {
	clients := &azureClients{cfg: cfg}
	if strings.TrimSpace(cfg.SubscriptionID) == "" {
		clients.initErr = fmt.Errorf("azure subscription_id is required")
		return clients
	}

	cred, err := createAzureCredential(cfg)
	if err != nil {
		clients.initErr = err
		return clients
	}
	clients.cred = cred

	securityGroupsClient, err := newSecurityGroupsClient(cfg.SubscriptionID, cred)
	if err != nil {
		clients.initErr = fmt.Errorf("create azure security groups client: %w", err)
		return clients
	}
	routeTablesClient, err := newRouteTablesClient(cfg.SubscriptionID, cred)
	if err != nil {
		clients.initErr = fmt.Errorf("create azure route tables client: %w", err)
		return clients
	}
	loadBalancersClient, err := newLoadBalancersClient(cfg.SubscriptionID, cred)
	if err != nil {
		clients.initErr = fmt.Errorf("create azure load balancers client: %w", err)
		return clients
	}
	loadBalancerProbesClient, err := newLoadBalancerProbesClient(cfg.SubscriptionID, cred)
	if err != nil {
		clients.initErr = fmt.Errorf("create azure load balancer probes client: %w", err)
		return clients
	}
	applicationGatewaysClient, err := newApplicationGatewaysClient(cfg.SubscriptionID, cred)
	if err != nil {
		clients.initErr = fmt.Errorf("create azure application gateways client: %w", err)
		return clients
	}

	activityClient, err := newActivityLogsClient(cfg.SubscriptionID, cred)
	if err != nil {
		clients.initErr = fmt.Errorf("create azure activity logs client: %w", err)
		return clients
	}

	logsClient, err := newLogsClient(cred)
	if err != nil {
		clients.initErr = fmt.Errorf("create azure logs client: %w", err)
		return clients
	}

	clients.nsg = &sdkSecurityGroupClient{client: securityGroupsClient}
	clients.udr = &sdkRouteTableClient{client: routeTablesClient}
	clients.lb = &sdkLoadBalancerClient{client: loadBalancersClient}
	clients.lbProbes = &sdkLoadBalancerProbeClient{client: loadBalancerProbesClient}
	clients.appGateway = &sdkApplicationGatewayClient{client: applicationGatewaysClient}
	clients.logs = &sdkLogsClient{client: logsClient}
	clients.activity = &sdkActivityLogsClient{client: activityClient}

	return clients
}

func createAzureCredential(cfg config.AzureConfig) (azcore.TokenCredential, error) {
	if strings.TrimSpace(cfg.TenantID) != "" && strings.TrimSpace(cfg.ClientID) != "" && strings.TrimSpace(cfg.ClientSecret) != "" {
		cred, err := newClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, nil)
		if err != nil {
			return nil, fmt.Errorf("create client secret credential: %w", err)
		}
		return cred, nil
	}

	cred, err := newDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("create default azure credential: %w", err)
	}
	return cred, nil
}

type sdkSecurityGroupClient struct {
	client *armnetwork.SecurityGroupsClient
}

func (c *sdkSecurityGroupClient) ListAll(ctx context.Context) ([]securityGroupInfo, error) {
	pager := c.client.NewListAllPager(nil)
	var out []securityGroupInfo
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Value {
			out = append(out, flattenSecurityGroup(item))
		}
	}
	return out, nil
}

func (c *sdkSecurityGroupClient) ListByResourceGroup(ctx context.Context, resourceGroup string) ([]securityGroupInfo, error) {
	pager := c.client.NewListPager(resourceGroup, nil)
	var out []securityGroupInfo
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Value {
			out = append(out, flattenSecurityGroup(item))
		}
	}
	return out, nil
}

type sdkRouteTableClient struct{ client *armnetwork.RouteTablesClient }

func (c *sdkRouteTableClient) ListAll(ctx context.Context) ([]routeTableInfo, error) {
	pager := c.client.NewListAllPager(nil)
	var out []routeTableInfo
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Value {
			out = append(out, flattenRouteTable(item))
		}
	}
	return out, nil
}

func (c *sdkRouteTableClient) ListByResourceGroup(ctx context.Context, resourceGroup string) ([]routeTableInfo, error) {
	pager := c.client.NewListPager(resourceGroup, nil)
	var out []routeTableInfo
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Value {
			out = append(out, flattenRouteTable(item))
		}
	}
	return out, nil
}

type sdkLoadBalancerClient struct {
	client *armnetwork.LoadBalancersClient
}

func (c *sdkLoadBalancerClient) GetRules(ctx context.Context, resourceGroup, loadBalancerName string) ([]loadBalancerRuleInfo, error) {
	resp, err := c.client.Get(ctx, resourceGroup, loadBalancerName, nil)
	if err != nil {
		return nil, err
	}
	if resp.Properties == nil {
		return nil, nil
	}
	var out []loadBalancerRuleInfo
	for _, rule := range resp.Properties.LoadBalancingRules {
		out = append(out, flattenLoadBalancingRule(rule))
	}
	return out, nil
}

type sdkLoadBalancerProbeClient struct {
	client *armnetwork.LoadBalancerProbesClient
}

func (c *sdkLoadBalancerProbeClient) List(ctx context.Context, resourceGroup, loadBalancerName string) ([]loadBalancerProbeInfo, error) {
	pager := c.client.NewListPager(resourceGroup, loadBalancerName, nil)
	var out []loadBalancerProbeInfo
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, probe := range page.Value {
			out = append(out, flattenLoadBalancerProbe(probe))
		}
	}
	return out, nil
}

type sdkApplicationGatewayClient struct {
	client *armnetwork.ApplicationGatewaysClient
}

func (c *sdkApplicationGatewayClient) BackendHealth(ctx context.Context, resourceGroup, appGatewayName string) ([]appGatewayBackendPoolInfo, error) {
	poller, err := c.client.BeginBackendHealth(ctx, resourceGroup, appGatewayName, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	var out []appGatewayBackendPoolInfo
	for _, pool := range resp.BackendAddressPools {
		out = append(out, flattenAppGatewayBackendPool(pool))
	}
	return out, nil
}

func (c *sdkApplicationGatewayClient) GetListeners(ctx context.Context, resourceGroup, appGatewayName string) ([]appGatewayListenerInfo, error) {
	resp, err := c.client.Get(ctx, resourceGroup, appGatewayName, nil)
	if err != nil {
		return nil, err
	}
	if resp.Properties == nil {
		return nil, nil
	}
	var out []appGatewayListenerInfo
	for _, listener := range resp.Properties.HTTPListeners {
		out = append(out, flattenAppGatewayListener(listener))
	}
	return out, nil
}

type sdkLogsClient struct{ client *azquery.LogsClient }

func (c *sdkLogsClient) QueryWorkspace(ctx context.Context, workspaceID, query string, start, end time.Time) ([]logQueryTable, error) {
	resp, err := c.client.QueryWorkspace(ctx, workspaceID, azquery.Body{
		Query:    to.Ptr(query),
		Timespan: to.Ptr(azquery.NewTimeInterval(start, end)),
	}, nil)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	var tables []logQueryTable
	for _, table := range resp.Tables {
		out := logQueryTable{Name: stringValue(table.Name)}
		for _, col := range table.Columns {
			out.Columns = append(out.Columns, logQueryColumn{Name: stringValue(col.Name), Type: enumString(col.Type)})
		}
		for _, row := range table.Rows {
			mapped := map[string]any{}
			for idx, val := range row {
				name := strconv.Itoa(idx)
				if idx < len(out.Columns) {
					name = out.Columns[idx].Name
				}
				mapped[name] = val
			}
			out.Rows = append(out.Rows, mapped)
		}
		tables = append(tables, out)
	}

	return tables, nil
}

type sdkActivityLogsClient struct {
	client *armmonitor.ActivityLogsClient
}

func (c *sdkActivityLogsClient) List(ctx context.Context, filter string) ([]activityEventInfo, error) {
	pager := c.client.NewListPager(filter, nil)
	var out []activityEventInfo
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, event := range page.Value {
			out = append(out, flattenActivityEvent(event))
		}
	}
	return out, nil
}

func flattenSecurityGroup(item *armnetwork.SecurityGroup) securityGroupInfo {
	out := securityGroupInfo{
		ID:            stringValue(item.ID),
		Name:          stringValue(item.Name),
		Location:      stringValue(item.Location),
		ResourceGroup: resourceGroupFromID(stringValue(item.ID)),
	}
	if item.Properties == nil {
		return out
	}
	for _, rule := range item.Properties.SecurityRules {
		if rule == nil {
			continue
		}
		ruleInfo := securityGroupRule{Name: stringValue(rule.Name)}
		if rule.Properties != nil {
			ruleInfo.Description = stringValue(rule.Properties.Description)
			ruleInfo.Direction = enumString(rule.Properties.Direction)
			ruleInfo.Access = enumString(rule.Properties.Access)
			ruleInfo.Protocol = enumString(rule.Properties.Protocol)
			ruleInfo.Priority = int32Value(rule.Properties.Priority)
			ruleInfo.SourcePortRange = stringValue(rule.Properties.SourcePortRange)
			ruleInfo.DestinationPortRange = stringValue(rule.Properties.DestinationPortRange)
			ruleInfo.SourceAddressPrefix = stringValue(rule.Properties.SourceAddressPrefix)
			ruleInfo.DestinationAddressPrefix = stringValue(rule.Properties.DestinationAddressPrefix)
		}
		out.Rules = append(out.Rules, ruleInfo)
	}
	return out
}

func flattenRouteTable(item *armnetwork.RouteTable) routeTableInfo {
	out := routeTableInfo{
		ID:            stringValue(item.ID),
		Name:          stringValue(item.Name),
		Location:      stringValue(item.Location),
		ResourceGroup: resourceGroupFromID(stringValue(item.ID)),
	}
	if item.Properties == nil {
		return out
	}
	for _, route := range item.Properties.Routes {
		if route == nil {
			continue
		}
		info := routeInfo{Name: stringValue(route.Name)}
		if route.Properties != nil {
			info.AddressPrefix = stringValue(route.Properties.AddressPrefix)
			info.NextHopType = enumString(route.Properties.NextHopType)
			info.NextHopIP = stringValue(route.Properties.NextHopIPAddress)
		}
		out.Routes = append(out.Routes, info)
	}
	return out
}

func flattenLoadBalancingRule(rule *armnetwork.LoadBalancingRule) loadBalancerRuleInfo {
	out := loadBalancerRuleInfo{
		ID:   stringValue(rule.ID),
		Name: stringValue(rule.Name),
	}
	if rule.Properties == nil {
		return out
	}
	out.Protocol = enumString(rule.Properties.Protocol)
	out.FrontendPort = int32Value(rule.Properties.FrontendPort)
	out.BackendPort = int32Value(rule.Properties.BackendPort)
	out.FrontendIPConfigID = subResourceID(rule.Properties.FrontendIPConfiguration)
	out.BackendPoolID = subResourceID(rule.Properties.BackendAddressPool)
	out.ProbeID = subResourceID(rule.Properties.Probe)
	out.IdleTimeoutMinutes = int32Value(rule.Properties.IdleTimeoutInMinutes)
	out.EnableFloatingIP = boolValue(rule.Properties.EnableFloatingIP)
	out.DisableOutboundSNAT = boolValue(rule.Properties.DisableOutboundSnat)
	return out
}

func flattenLoadBalancerProbe(probe *armnetwork.Probe) loadBalancerProbeInfo {
	out := loadBalancerProbeInfo{ID: stringValue(probe.ID), Name: stringValue(probe.Name)}
	if probe.Properties == nil {
		return out
	}
	out.Protocol = enumString(probe.Properties.Protocol)
	out.Port = int32Value(probe.Properties.Port)
	out.RequestPath = stringValue(probe.Properties.RequestPath)
	out.IntervalSeconds = int32Value(probe.Properties.IntervalInSeconds)
	out.UnhealthyThreshold = int32Value(probe.Properties.NumberOfProbes)
	return out
}

func flattenAppGatewayListener(listener *armnetwork.ApplicationGatewayHTTPListener) appGatewayListenerInfo {
	out := appGatewayListenerInfo{ID: stringValue(listener.ID), Name: stringValue(listener.Name)}
	if listener.Properties == nil {
		return out
	}
	out.Protocol = enumString(listener.Properties.Protocol)
	out.HostName = stringValue(listener.Properties.HostName)
	out.HostNames = stringSlice(listener.Properties.HostNames)
	out.FrontendPortID = subResourceID(listener.Properties.FrontendPort)
	out.FrontendIPConfigID = subResourceID(listener.Properties.FrontendIPConfiguration)
	out.RequireServerNameIndication = boolValue(listener.Properties.RequireServerNameIndication)
	return out
}

func flattenAppGatewayBackendPool(pool *armnetwork.ApplicationGatewayBackendHealthPool) appGatewayBackendPoolInfo {
	out := appGatewayBackendPoolInfo{}
	if pool == nil {
		return out
	}
	if pool.BackendAddressPool != nil {
		out.ID = stringValue(pool.BackendAddressPool.ID)
		out.Name = stringValue(pool.BackendAddressPool.Name)
	}
	for _, setting := range pool.BackendHTTPSettingsCollection {
		if setting == nil {
			continue
		}
		settingInfo := appGatewayBackendHTTPSettingsInfo{}
		if setting.BackendHTTPSettings != nil {
			settingInfo.ID = stringValue(setting.BackendHTTPSettings.ID)
			settingInfo.Name = stringValue(setting.BackendHTTPSettings.Name)
		}
		for _, server := range setting.Servers {
			if server == nil {
				continue
			}
			settingInfo.Servers = append(settingInfo.Servers, appGatewayBackendServerInfo{
				Address: stringValue(server.Address),
				Health:  enumString(server.Health),
			})
		}
		out.HTTPSettings = append(out.HTTPSettings, settingInfo)
	}
	return out
}

func flattenActivityEvent(event *armmonitor.EventData) activityEventInfo {
	out := activityEventInfo{
		Caller:        stringValue(event.Caller),
		CorrelationID: stringValue(event.CorrelationID),
		Level:         enumString(event.Level),
		ResourceGroup: stringValue(event.ResourceGroupName),
		ResourceID:    stringValue(event.ResourceID),
		SubStatus:     localizedValue(event.SubStatus),
		Description:   localizedValue(event.Description),
	}
	if event.EventTimestamp != nil {
		out.EventTimestamp = event.EventTimestamp.Format(time.RFC3339)
	}
	if event.OperationName != nil {
		out.OperationName = localizedValue(event.OperationName)
	}
	if event.Status != nil {
		out.Status = localizedValue(event.Status)
	}
	if event.Category != nil {
		out.Category = localizedValue(event.Category)
	}
	if event.ResourceProviderName != nil {
		out.ResourceType = localizedValue(event.ResourceProviderName)
	}
	return out
}

func requireString(params map[string]any, key string) (string, error) {
	v, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	return strings.TrimSpace(s), nil
}

func optionalString(params map[string]any, key, defaultValue string) (string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultValue, nil
	}
	return s, nil
}

func optionalBool(params map[string]any, key string, defaultValue bool) (bool, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("parameter %s must be a boolean", key)
	}
	return b, nil
}

func optionalInt(params map[string]any, key string, defaultValue int) (int, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return defaultValue, nil
	}
	switch value := v.(type) {
	case int:
		return value, nil
	case int32:
		return int(value), nil
	case int64:
		return int(value), nil
	case float64:
		if value != float64(int(value)) {
			return 0, fmt.Errorf("parameter %s must be an integer", key)
		}
		return int(value), nil
	default:
		return 0, fmt.Errorf("parameter %s must be an integer", key)
	}
}

func normalizeMaxRows(params map[string]any) (int, error) {
	maxRows, err := common.OptionalInt(params, "max_rows", 100)
	if err != nil {
		return 0, err
	}
	if maxRows <= 0 {
		return 0, fmt.Errorf("parameter max_rows must be greater than 0")
	}
	if maxRows > 1000 {
		maxRows = 1000
	}
	return maxRows, nil
}

func resolveWorkspaceID(cfg config.AzureConfig, params map[string]any) (string, error) {
	workspaceID, err := common.OptionalString(params, "workspace_id", strings.TrimSpace(cfg.WorkspaceID))
	if err != nil {
		return "", err
	}
	if workspaceID == "" {
		return "", fmt.Errorf("missing required parameter: workspace_id")
	}
	return workspaceID, nil
}

func parseTimespan(params map[string]any, key, defaultValue string) (time.Time, time.Time, string, error) {
	raw, err := common.OptionalString(params, key, defaultValue)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	goDuration := raw
	if strings.HasPrefix(strings.ToUpper(raw), "P") {
		goDuration, err = iso8601DurationToGo(raw)
		if err != nil {
			return time.Time{}, time.Time{}, "", err
		}
	}
	duration, err := time.ParseDuration(goDuration)
	if err != nil {
		return time.Time{}, time.Time{}, "", fmt.Errorf("parameter %s must be a valid duration or ISO-8601 duration", key)
	}
	end := time.Now().UTC()
	start := end.Add(-duration)
	return start, end, raw, nil
}

func iso8601DurationToGo(value string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if upper == "" || !strings.HasPrefix(upper, "P") {
		return "", fmt.Errorf("invalid ISO-8601 duration")
	}
	upper = strings.TrimPrefix(upper, "P")
	dayPart, timePart, hasTimePart := strings.Cut(upper, "T")
	if !hasTimePart {
		dayPart = upper
		timePart = ""
	}

	var parts []string
	if dayPart != "" {
		days, rest, err := consumeISOPart(dayPart, 'D')
		if err != nil || rest != "" {
			return "", fmt.Errorf("invalid ISO-8601 duration")
		}
		if days > 0 {
			parts = append(parts, fmt.Sprintf("%dh", days*24))
		}
	}
	remaining := timePart
	for _, unit := range []struct {
		marker byte
		suffix string
	}{
		{'H', "h"},
		{'M', "m"},
		{'S', "s"},
	} {
		amount, rest, err := consumeISOPart(remaining, unit.marker)
		if err != nil {
			return "", fmt.Errorf("invalid ISO-8601 duration")
		}
		remaining = rest
		if amount > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", amount, unit.suffix))
		}
	}
	if remaining != "" || len(parts) == 0 {
		return "", fmt.Errorf("invalid ISO-8601 duration")
	}
	return strings.Join(parts, ""), nil
}

func consumeISOPart(input string, marker byte) (int, string, error) {
	idx := strings.IndexByte(input, marker)
	if idx < 0 {
		return 0, input, nil
	}
	start := idx - 1
	for start >= 0 && input[start] >= '0' && input[start] <= '9' {
		start--
	}
	start++
	if start == idx {
		return 0, "", fmt.Errorf("missing numeric part")
	}
	amount, err := strconv.Atoi(input[start:idx])
	if err != nil {
		return 0, "", err
	}
	return amount, input[:start] + input[idx+1:], nil
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func int32Value(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

func boolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func stringSlice(values []*string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil && *value != "" {
			out = append(out, *value)
		}
	}
	return out
}

func subResourceID(v *armnetwork.SubResource) string {
	if v == nil {
		return ""
	}
	return stringValue(v.ID)
}

func resourceGroupFromID(id string) string {
	parts := strings.Split(id, "/")
	for idx, part := range parts {
		if strings.EqualFold(part, "resourceGroups") && idx+1 < len(parts) {
			return parts[idx+1]
		}
	}
	return ""
}

func enumString(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		return fmt.Sprint(rv.Elem().Interface())
	}
	return fmt.Sprint(v)
}

func localizedValue(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		field := rv.FieldByName("LocalizedValue")
		if field.IsValid() && field.Kind() == reflect.Pointer && !field.IsNil() {
			if s, ok := field.Interface().(*string); ok {
				return stringValue(s)
			}
		}
		field = rv.FieldByName("Value")
		if field.IsValid() && field.Kind() == reflect.Pointer && !field.IsNil() {
			if s, ok := field.Interface().(*string); ok {
				return stringValue(s)
			}
		}
	}
	return fmt.Sprint(rv.Interface())
}
