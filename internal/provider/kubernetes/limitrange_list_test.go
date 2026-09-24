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

func TestLimitRangeListTask(t *testing.T) {
	t.Parallel()

	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cpu-memory-limits",
			Namespace: "default",
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(lr)})
	task := &limitRangeListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	ranges := data["limit_ranges"].([]limitRangeSummary)
	require.Len(t, ranges, 1)
	assert.Equal(t, "cpu-memory-limits", ranges[0].Name)
	assert.Equal(t, "default", ranges[0].Namespace)
}

func TestLimitRangeListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &limitRangeListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestLimitRangeGetTask(t *testing.T) {
	t.Parallel()

	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cpu-memory-limits",
			Namespace: "default",
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Max: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
					Min: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(lr)})
	task := &limitRangeGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "cpu-memory-limits"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	l := data["limit_range"].(limitRangeDetail)
	assert.Equal(t, "cpu-memory-limits", l.Name)
	assert.Equal(t, "default", l.Namespace)
	require.Len(t, l.Limits, 1)
	assert.Equal(t, "Container", l.Limits[0].Type)
	assert.Equal(t, "500m", l.Limits[0].Default["cpu"])
	assert.Equal(t, "256Mi", l.Limits[0].Default["memory"])
	assert.Equal(t, "100m", l.Limits[0].DefaultRequest["cpu"])
	assert.Equal(t, "128Mi", l.Limits[0].DefaultRequest["memory"])
	assert.Equal(t, "2", l.Limits[0].Max["cpu"])
	assert.Equal(t, "1Gi", l.Limits[0].Max["memory"])
	assert.Equal(t, "50m", l.Limits[0].Min["cpu"])
	assert.Equal(t, "64Mi", l.Limits[0].Min["memory"])
}

func TestLimitRangeGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &limitRangeGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestLimitRangeGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &limitRangeGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
