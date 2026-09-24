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

func TestReplicaSetListTask(t *testing.T) {
	t.Parallel()

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-7c4b8f5d9", Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "nginx"}},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: 3, ReadyReplicas: 3, FullyLabeledReplicas: 3},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(rs)})
	task := &replicaSetListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	items := data["replicasets"].([]replicaSetSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "nginx-7c4b8f5d9", items[0].Name)
	assert.Equal(t, 3, items[0].Replicas)
	assert.Equal(t, "Deployment/nginx", items[0].OwnerReference)
}

func TestReplicaSetGetTask(t *testing.T) {
	t.Parallel()

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-7c4b8f5d9", Namespace: "default",
			Labels:          map[string]string{"app": "nginx"},
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "nginx"}},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: 3, ReadyReplicas: 3, FullyLabeledReplicas: 3},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(rs)})
	task := &replicaSetGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "nginx-7c4b8f5d9"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	d := data["replicaset"].(replicaSetDetail)
	assert.Equal(t, "nginx-7c4b8f5d9", d.Name)
	assert.Equal(t, "Deployment/nginx", d.OwnerReference)
	assert.Equal(t, map[string]string{"app": "nginx"}, d.Labels)
}
