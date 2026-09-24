package azure

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeActivityLogsClient struct {
	filter string
	events []activityEventInfo
}

func (f *fakeActivityLogsClient) List(_ context.Context, filter string) ([]activityEventInfo, error) {
	f.filter = filter
	return f.events, nil
}

func TestActivityChangesTaskExecute(t *testing.T) {
	t.Parallel()

	client := &fakeActivityLogsClient{events: []activityEventInfo{{OperationName: "Microsoft.Network/networkSecurityGroups/write"}}}
	p := newTestProvider(&azureClients{activity: client})
	task := &activityChangesTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"resource_group": "rg1", "operation_name": "write", "max_rows": 1})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Contains(t, client.filter, "resourceGroupName eq 'rg1'")
	assert.Contains(t, client.filter, "operationName/value eq 'write'")
	assert.Len(t, res.Data.(map[string]any)["events"].([]activityEventInfo), 1)
}
