package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLoadBalancerClient struct{ rules []loadBalancerRuleInfo }

func (f *fakeLoadBalancerClient) GetRules(_ context.Context, _, _ string) ([]loadBalancerRuleInfo, error) {
	return f.rules, nil
}

func TestLBRulesTaskExecute(t *testing.T) {
	t.Parallel()

	p := newTestProvider(&azureClients{lb: &fakeLoadBalancerClient{rules: []loadBalancerRuleInfo{{Name: "rule1", FrontendPort: 80}}}})
	task := &lbRulesTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"resource_group": "rg1", "load_balancer_name": "lb1"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Len(t, res.Data.(map[string]any)["rules"].([]loadBalancerRuleInfo), 1)
}

func TestLBRulesTaskRequiresParams(t *testing.T) {
	t.Parallel()

	p := &Provider{}
	task := &lbRulesTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "resource_group")
}
