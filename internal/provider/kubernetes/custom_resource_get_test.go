package kubernetes

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomResourceGetTask(t *testing.T) {
	t.Parallel()

	certificate := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      "site-cert",
			"namespace": "default",
		},
		"status": map[string]any{"ready": true},
	}}
	certificate.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"})
	provider := newTestProvider(&k8sClient{dynamicClient: fake.NewSimpleDynamicClient(runtime.NewScheme(), certificate)})
	task := &customResourceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{
		"api_group": "cert-manager.io",
		"version":   "v1",
		"resource":  "certificates",
		"namespace": "default",
		"name":      "site-cert",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	object := data["object"].(map[string]any)
	metadata := object["metadata"].(map[string]any)
	assert.Equal(t, "site-cert", metadata["name"])
	assert.Equal(t, true, object["status"].(map[string]any)["ready"])
}

func TestCustomResourceGetTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{dynamicClient: fake.NewSimpleDynamicClient(runtime.NewScheme())})
	task := &customResourceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{
		"api_group": "cert-manager.io",
		"version":   "v1",
		"resource":  "certificates",
		"namespace": "default",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestCustomResourceGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &customResourceGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{
		"api_group": "cert-manager.io",
		"version":   "v1",
		"resource":  "certificates",
		"namespace": "default",
		"name":      "site-cert",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}
