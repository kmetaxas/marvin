package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLoadBalancerProbeClient struct{ probes []loadBalancerProbeInfo }

func (f *fakeLoadBalancerProbeClient) List(_ context.Context, _, _ string) ([]loadBalancerProbeInfo, error) {
	return f.probes, nil
}

func TestLBHealthTaskExecute(t *testing.T) {
	t.Parallel()

	p := newTestProvider(&azureClients{lbProbes: &fakeLoadBalancerProbeClient{probes: []loadBalancerProbeInfo{{Name: "probe1", Port: 443}}}})
	task := &lbHealthTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"resource_group": "rg1", "load_balancer_name": "lb1"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Len(t, res.Data.(map[string]any)["probes"].([]loadBalancerProbeInfo), 1)
}
