package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatefulSetListTask(t *testing.T) {
	t.Parallel()

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appsv1.StatefulSetSpec{ServiceName: "web-headless"},
		Status:     appsv1.StatefulSetStatus{Replicas: 3, ReadyReplicas: 3, UpdatedReplicas: 3},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ss)})
	task := &statefulSetListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	items := data["statefulsets"].([]statefulSetSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "web", items[0].Name)
	assert.Equal(t, "web-headless", items[0].ServiceName)
	assert.Equal(t, 3, items[0].ReadyReplicas)
}

func TestStatefulSetGetTask(t *testing.T) {
	t.Parallel()

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{"app": "web"}},
		Spec:       appsv1.StatefulSetSpec{ServiceName: "web-headless"},
		Status:     appsv1.StatefulSetStatus{Replicas: 3, ReadyReplicas: 3, UpdatedReplicas: 3},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ss)})
	task := &statefulSetGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "web"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	d := data["statefulset"].(statefulSetDetail)
	assert.Equal(t, "web", d.Name)
	assert.Equal(t, "web-headless", d.ServiceName)
	assert.Equal(t, 3, d.UpdatedReplicas)
	assert.Equal(t, map[string]string{"app": "web"}, d.Labels)
}
