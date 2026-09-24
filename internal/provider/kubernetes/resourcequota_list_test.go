package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceQuotaListTask(t *testing.T) {
	t.Parallel()

	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compute-quota",
			Namespace: "default",
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(rq)})
	task := &resourceQuotaListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	quotas := data["resource_quotas"].([]resourceQuotaSummary)
	require.Len(t, quotas, 1)
	assert.Equal(t, "compute-quota", quotas[0].Name)
	assert.Equal(t, "default", quotas[0].Namespace)
}

func TestResourceQuotaListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &resourceQuotaListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestResourceQuotaGetTask(t *testing.T) {
	t.Parallel()

	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compute-quota",
			Namespace: "default",
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("4"),
				corev1.ResourceRequestsMemory: resource.MustParse("8Gi"),
			},
			Used: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("2"),
				corev1.ResourceRequestsMemory: resource.MustParse("4Gi"),
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(rq)})
	task := &resourceQuotaGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "compute-quota"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	q := data["resource_quota"].(resourceQuotaDetail)
	assert.Equal(t, "compute-quota", q.Name)
	assert.Equal(t, "default", q.Namespace)
	assert.Equal(t, "4", q.Hard["requests.cpu"])
	assert.Equal(t, "8Gi", q.Hard["requests.memory"])
	assert.Equal(t, "2", q.Used["requests.cpu"])
	assert.Equal(t, "4Gi", q.Used["requests.memory"])
}

func TestResourceQuotaGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &resourceQuotaGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestResourceQuotaGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &resourceQuotaGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
