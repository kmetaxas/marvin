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

func TestConfigMapListTask(t *testing.T) {
	t.Parallel()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-config",
			Namespace: "default",
		},
		Data: map[string]string{"nginx.conf": "server { listen 80; }"},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(cm)})
	task := &configMapListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	configMaps := data["config_maps"].([]configMapSummary)
	require.Len(t, configMaps, 1)
	assert.Equal(t, "nginx-config", configMaps[0].Name)
	assert.Equal(t, "default", configMaps[0].Namespace)
	assert.Equal(t, []string{"nginx.conf"}, configMaps[0].DataKeys)
}

func TestConfigMapListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &configMapListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestConfigMapGetTask(t *testing.T) {
	t.Parallel()

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-config",
			Namespace: "default",
		},
		Data: map[string]string{"nginx.conf": "server { listen 80; }"},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(cm)})
	task := &configMapGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "nginx-config"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	c := data["config_map"].(configMapDetail)
	assert.Equal(t, "nginx-config", c.Name)
	assert.Equal(t, "default", c.Namespace)
	assert.Equal(t, "server { listen 80; }", c.Data["nginx.conf"])
}

func TestConfigMapGetTaskTruncation(t *testing.T) {
	t.Parallel()

	largeValue := make([]byte, maxConfigMapDataValue+10)
	for i := range largeValue {
		largeValue[i] = 'a'
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "big-config",
			Namespace: "default",
		},
		Data: map[string]string{"key1": string(largeValue)},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(cm)})
	task := &configMapGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "big-config"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	c := data["config_map"].(configMapDetail)
	assert.Equal(t, maxConfigMapDataValue, len(c.Data["key1"].(string)))
	assert.Equal(t, true, c.Data["key1_truncated"])
}

func TestConfigMapGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &configMapGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestConfigMapGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &configMapGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
