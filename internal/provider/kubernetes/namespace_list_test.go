package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamespaceListTask(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "default",
			Labels: map[string]string{"kubernetes.io/metadata.name": "default"},
		},
		Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ns)})
	task := &namespaceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	namespaces := data["namespaces"].([]namespaceSummary)
	require.Len(t, namespaces, 1)
	assert.Equal(t, "default", namespaces[0].Name)
	assert.Equal(t, "Active", namespaces[0].Status)
	assert.Equal(t, map[string]string{"kubernetes.io/metadata.name": "default"}, namespaces[0].Labels)
}

func TestNamespaceListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &namespaceListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestNamespaceGetTask(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "default",
			Labels: map[string]string{"kubernetes.io/metadata.name": "default"},
		},
		Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ns)})
	task := &namespaceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	n := data["namespace"].(namespaceSummary)
	assert.Equal(t, "default", n.Name)
	assert.Equal(t, "Active", n.Status)
}

func TestNamespaceGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &namespaceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestNamespaceGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &namespaceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
