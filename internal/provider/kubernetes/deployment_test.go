package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentListTask(t *testing.T) {
	t.Parallel()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
			Labels:    map[string]string{"app": "nginx"},
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          3,
			ReadyReplicas:     3,
			UpdatedReplicas:   3,
			AvailableReplicas: 3,
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(dep)})
	task := &deploymentListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	items := data["deployments"].([]deploymentSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "nginx", items[0].Name)
	assert.Equal(t, 3, items[0].Replicas)
	assert.Equal(t, 3, items[0].ReadyReplicas)
	assert.Equal(t, "RollingUpdate", items[0].Strategy)
}

func TestDeploymentGetTask(t *testing.T) {
	t.Parallel()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx", Namespace: "default",
			Labels: map[string]string{"app": "nginx"},
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
		},
		Status: appsv1.DeploymentStatus{
			Replicas: 3, ReadyReplicas: 3, UpdatedReplicas: 3, AvailableReplicas: 3,
			Conditions: []appsv1.DeploymentCondition{
				{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(dep)})
	task := &deploymentGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "nginx"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	d := data["deployment"].(deploymentDetail)
	assert.Equal(t, "nginx", d.Name)
	assert.Equal(t, "RollingUpdate", d.Strategy)
	assert.Equal(t, map[string]string{"app": "nginx"}, d.Selector)
	require.Len(t, d.Conditions, 1)
	assert.Equal(t, "Progressing", d.Conditions[0].Type)
}
