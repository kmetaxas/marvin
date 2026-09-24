package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAPIResourcesListTask(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	// Fake discovery returns some default resources; just verify the call succeeds.
	task := &apiResourcesListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.GreaterOrEqual(t, data["count"], 0)
}

func TestAPIResourcesListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &apiResourcesListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestAPIResourcesListTaskWithFilter(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &apiResourcesListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"api_group": "", "namespaced": true})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.GreaterOrEqual(t, data["count"], 0)
}
