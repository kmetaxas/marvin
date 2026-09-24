package azure

import (
	"context"
	"testing"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVNetFlowLogsTaskExecute(t *testing.T) {
	t.Parallel()

	client := &fakeLogsClient{tables: []logQueryTable{{Rows: []map[string]any{{"NSG_s": "nsg1"}}}}}
	p := newTestProvider(&azureClients{logs: client}, config.AzureConfig{WorkspaceID: "workspace1"})
	task := &vnetFlowLogsTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"nsg_name": "nsg1", "source_ip": "10.0.0.1", "dest_ip": "10.0.0.2"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, client.query, "AzureNetworkAnalytics_CL")
	assert.Contains(t, client.query, "| take 100")
	assert.Contains(t, client.query, "10.0.0.1")
}
