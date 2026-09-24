package kubernetes

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomResourceListTask(t *testing.T) {
	t.Parallel()

	gvr := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}
	certificate := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cert-manager.io/v1",
		"kind":       "Certificate",
		"metadata": map[string]any{
			"name":      "site-cert",
			"namespace": "default",
		},
		"spec": map[string]any{"dnsNames": []any{"example.com"}},
	}}
	certificate.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"})
	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "CertificateList"},
		certificate,
	)
	provider := newTestProvider(&k8sClient{dynamicClient: dynamicClient})
	task := &customResourceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{
		"api_group": "cert-manager.io",
		"version":   "v1",
		"resource":  "certificates",
		"namespace": "default",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	items := data["items"].([]map[string]any)
	require.Len(t, items, 1)
	metadata := items[0]["metadata"].(map[string]any)
	assert.Equal(t, "site-cert", metadata["name"])
	assert.Equal(t, "default", metadata["namespace"])
}

func TestCustomResourceListTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{dynamicClient: fake.NewSimpleDynamicClient(runtime.NewScheme())})
	task := &customResourceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"api_group": "cert-manager.io", "version": "v1"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "resource")
}

func TestCustomResourceListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &customResourceListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{
		"api_group": "cert-manager.io",
		"version":   "v1",
		"resource":  "certificates",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestCustomResourceListTaskInvalidLimit(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{dynamicClient: fake.NewSimpleDynamicClient(runtime.NewScheme())})
	task := &customResourceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{
		"api_group": "cert-manager.io",
		"version":   "v1",
		"resource":  "certificates",
		"limit":     0,
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit")
}

var _ metav1.Object = (*unstructured.Unstructured)(nil)
