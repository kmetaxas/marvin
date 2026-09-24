package azure

import (
	"context"
	"testing"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLogsClient struct {
	workspaceID string
	query       string
	start       time.Time
	end         time.Time
	tables      []logQueryTable
}

func (f *fakeLogsClient) QueryWorkspace(_ context.Context, workspaceID, query string, start, end time.Time) ([]logQueryTable, error) {
	f.workspaceID = workspaceID
	f.query = query
	f.start = start
	f.end = end
	return f.tables, nil
}

func TestAppGatewayLogsTaskExecute(t *testing.T) {
	t.Parallel()

	client := &fakeLogsClient{tables: []logQueryTable{{Rows: []map[string]any{{"Resource": "agw1"}}}}}
	p := newTestProvider(&azureClients{logs: client}, config.AzureConfig{WorkspaceID: "workspace-default"})
	task := &appGatewayLogsTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"app_gateway_name": "agw1", "max_rows": 2000, "timespan": "1h"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "workspace-default", client.workspaceID)
	assert.Contains(t, client.query, "| take 1000")
	assert.Contains(t, client.query, "agw1")
	assert.False(t, client.start.IsZero())
	assert.False(t, client.end.IsZero())
}
