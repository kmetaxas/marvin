package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApplicationGatewayClient struct {
	backendPools []appGatewayBackendPoolInfo
	listeners    []appGatewayListenerInfo
}

func (f *fakeApplicationGatewayClient) BackendHealth(_ context.Context, _, _ string) ([]appGatewayBackendPoolInfo, error) {
	return f.backendPools, nil
}

func (f *fakeApplicationGatewayClient) GetListeners(_ context.Context, _, _ string) ([]appGatewayListenerInfo, error) {
	return f.listeners, nil
}

func TestAppGatewayBackendHealthTaskExecute(t *testing.T) {
	t.Parallel()

	p := newTestProvider(&azureClients{appGateway: &fakeApplicationGatewayClient{backendPools: []appGatewayBackendPoolInfo{{Name: "pool1"}}}})
	task := &appGatewayBackendHealthTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"resource_group": "rg1", "app_gateway_name": "agw1"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Len(t, res.Data.(map[string]any)["backend_pools"].([]appGatewayBackendPoolInfo), 1)
}
