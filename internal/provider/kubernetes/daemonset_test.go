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

func TestDaemonSetListTask(t *testing.T) {
	t.Parallel()

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "fluentd", Namespace: "kube-system"},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 5,
			CurrentNumberScheduled: 5,
			NumberReady:            5,
			UpdatedNumberScheduled: 5,
			NumberAvailable:        5,
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ds)})
	task := &daemonSetListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "kube-system"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	items := data["daemonsets"].([]daemonSetSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "fluentd", items[0].Name)
	assert.Equal(t, 5, items[0].Desired)
	assert.Equal(t, 5, items[0].Ready)
	assert.Equal(t, 5, items[0].Available)
}

func TestDaemonSetGetTask(t *testing.T) {
	t.Parallel()

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "fluentd", Namespace: "kube-system", Labels: map[string]string{"app": "fluentd"}},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 5, CurrentNumberScheduled: 5,
			NumberReady: 5, UpdatedNumberScheduled: 5, NumberAvailable: 5,
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ds)})
	task := &daemonSetGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "kube-system", "name": "fluentd"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	d := data["daemonset"].(daemonSetDetail)
	assert.Equal(t, "fluentd", d.Name)
	assert.Equal(t, 5, d.Desired)
	assert.Equal(t, 5, d.Ready)
	assert.Equal(t, map[string]string{"app": "fluentd"}, d.Labels)
}
