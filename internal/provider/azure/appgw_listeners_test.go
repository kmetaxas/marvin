package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppGatewayListenersTaskExecute(t *testing.T) {
	t.Parallel()

	p := newTestProvider(&azureClients{appGateway: &fakeApplicationGatewayClient{listeners: []appGatewayListenerInfo{{Name: "listener1", HostName: "example.com"}}}})
	task := &appGatewayListenersTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"resource_group": "rg1", "app_gateway_name": "agw1"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Len(t, res.Data.(map[string]any)["listeners"].([]appGatewayListenerInfo), 1)
}
