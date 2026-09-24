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

func TestPodGetFullTask(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "api",
			Namespace:   "default",
			Labels:      map[string]string{"app": "api"},
			Annotations: map[string]string{"debug": "true"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: "nginx:latest"}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.244.0.10",
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(pod)})
	task := &podGetFullTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	rawPod := data["pod"].(map[string]any)
	metadata := rawPod["metadata"].(map[string]any)
	spec := rawPod["spec"].(map[string]any)
	status := rawPod["status"].(map[string]any)
	assert.Equal(t, "api", metadata["name"])
	assert.Equal(t, "default", metadata["namespace"])
	assert.Equal(t, "Running", status["phase"])
	assert.Equal(t, "10.244.0.10", status["podIP"])
	containers := spec["containers"].([]any)
	require.Len(t, containers, 1)
	assert.Equal(t, "nginx:latest", containers[0].(map[string]any)["image"])
}

func TestPodGetFullTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &podGetFullTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestPodGetFullTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &podGetFullTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}
