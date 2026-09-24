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

func TestServiceAccountListTask(t *testing.T) {
	t.Parallel()

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Secrets: []corev1.ObjectReference{
			{Name: "default-token-abc123"},
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "regcred"},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(sa)})
	task := &serviceAccountListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	accounts := data["service_accounts"].([]serviceAccountSummary)
	require.Len(t, accounts, 1)
	assert.Equal(t, "default", accounts[0].Name)
	assert.Equal(t, "default", accounts[0].Namespace)
	assert.Equal(t, []string{"default-token-abc123"}, accounts[0].Secrets)
	assert.Equal(t, []string{"regcred"}, accounts[0].ImagePullSecrets)
}

func TestServiceAccountListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &serviceAccountListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestServiceAccountGetTask(t *testing.T) {
	t.Parallel()

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
		Secrets: []corev1.ObjectReference{
			{Name: "default-token-abc123"},
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "regcred"},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(sa)})
	task := &serviceAccountGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	a := data["service_account"].(serviceAccountSummary)
	assert.Equal(t, "default", a.Name)
	assert.Equal(t, []string{"default-token-abc123"}, a.Secrets)
	assert.Equal(t, []string{"regcred"}, a.ImagePullSecrets)
}

func TestServiceAccountGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &serviceAccountGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestServiceAccountGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &serviceAccountGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
