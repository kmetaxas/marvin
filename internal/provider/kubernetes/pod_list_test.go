package kubernetes

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPodListTask(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx"},
		},
		Spec: corev1.PodSpec{
			NodeName:           "node-1",
			ServiceAccountName: "default",
			RestartPolicy:      corev1.RestartPolicyAlways,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 0},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(pod)})
	task := &podListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	pods := data["pods"].([]podSummary)
	require.Len(t, pods, 1)
	assert.Equal(t, "nginx", pods[0].Name)
	assert.Equal(t, "default", pods[0].Namespace)
	assert.Equal(t, "Running", pods[0].Status)
	assert.Equal(t, "1/1", pods[0].ReadyContainers)
	assert.Equal(t, 0, pods[0].Restarts)
	assert.Equal(t, "node-1", pods[0].NodeName)
}

func TestPodListTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &podListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"limit": "notanint"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit")
}

func TestPodListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &podListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestPodListTaskAllNamespaces(t *testing.T) {
	t.Parallel()

	pod1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	pod2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns2"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(pod1, pod2)})
	task := &podListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}

func TestPodGetTask(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx"},
			UID:       "test-uid",
		},
		Spec: corev1.PodSpec{
			NodeName:           "node-1",
			ServiceAccountName: "default",
			RestartPolicy:      corev1.RestartPolicyAlways,
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:1.25"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "nginx", Ready: true, RestartCount: 0, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}}}},
			},
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
			PodIP: "10.0.0.1",
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(pod)})
	task := &podGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "nginx"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	p := data["pod"].(podDetail)
	assert.Equal(t, "nginx", p.Name)
	assert.Equal(t, "default", p.Namespace)
	assert.Equal(t, "test-uid", p.UID)
	assert.Equal(t, "node-1", p.NodeName)
	assert.Equal(t, "10.0.0.1", p.PodIP)
	assert.Equal(t, "Running", p.Phase)
	require.Len(t, p.Containers, 1)
	assert.Equal(t, "nginx", p.Containers[0].Name)
	assert.Equal(t, "nginx:1.25", p.Containers[0].Image)
	assert.True(t, p.Containers[0].Ready)
	assert.Equal(t, "running", p.Containers[0].State)
	require.Len(t, p.Conditions, 1)
	assert.Equal(t, "Ready", p.Conditions[0].Type)
	assert.Equal(t, "True", p.Conditions[0].Status)
}

func TestPodGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &podGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestPodGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &podGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
