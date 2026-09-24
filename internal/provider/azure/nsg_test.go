package azure

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSecurityGroupClient struct {
	listAllResp []securityGroupInfo
	listRGResp  []securityGroupInfo
	err         error
	lastRG      string
	usedListAll bool
}

func (f *fakeSecurityGroupClient) ListAll(context.Context) ([]securityGroupInfo, error) {
	f.usedListAll = true
	return f.listAllResp, f.err
}

func (f *fakeSecurityGroupClient) ListByResourceGroup(_ context.Context, rg string) ([]securityGroupInfo, error) {
	f.lastRG = rg
	return f.listRGResp, f.err
}

func TestNSGTaskExecute(t *testing.T) {
	t.Parallel()

	client := &fakeSecurityGroupClient{listAllResp: []securityGroupInfo{{Name: "nsg-a", Rules: []securityGroupRule{{Name: "allow-443"}}}}}
	p := newTestProvider(&azureClients{nsg: client})
	task := &nsgTask{provider: p}

	res, err := task.Execute(context.Background(), map[string]any{"include_rules": false})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.True(t, client.usedListAll)
	data := res.Data.(map[string]any)
	items := data["network_security_groups"].([]securityGroupInfo)
	assert.Empty(t, items[0].Rules)
}

func TestNSGTaskHandlesAzureError(t *testing.T) {
	t.Parallel()

	p := newTestProvider(&azureClients{nsg: &fakeSecurityGroupClient{err: errors.New("api failed")}})
	task := &nsgTask{provider: p}
	res, err := task.Execute(context.Background(), map[string]any{"resource_group": "rg1"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}
