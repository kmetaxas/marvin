package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRouteTableClient struct {
	listAllResp []routeTableInfo
	listRGResp  []routeTableInfo
	lastRG      string
}

func (f *fakeRouteTableClient) ListAll(context.Context) ([]routeTableInfo, error) {
	return f.listAllResp, nil
}
func (f *fakeRouteTableClient) ListByResourceGroup(_ context.Context, rg string) ([]routeTableInfo, error) {
	f.lastRG = rg
	return f.listRGResp, nil
}

func TestUDRTaskExecute(t *testing.T) {
	t.Parallel()

	client := &fakeRouteTableClient{listRGResp: []routeTableInfo{{Name: "rt1", Routes: []routeInfo{{Name: "default"}}}}}
	p := newTestProvider(&azureClients{udr: client})
	task := &udrTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"resource_group": "rg1", "include_routes": false})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "rg1", client.lastRG)
	items := res.Data.(map[string]any)["route_tables"].([]routeTableInfo)
	assert.Empty(t, items[0].Routes)
}
