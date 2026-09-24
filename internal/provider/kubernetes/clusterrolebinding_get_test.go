package kubernetes

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterRoleBindingGetTask(t *testing.T) {
	t.Parallel()

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-admin-binding",
		},
		RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "marvin", Namespace: "default"},
			{Kind: "User", Name: "alice"},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(clusterRoleBinding)})
	task := &clusterRoleBindingGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "cluster-admin-binding"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	clusterRoleBindingDetail := data["cluster_role_binding"].(clusterRoleBindingDetail)
	assert.Equal(t, "cluster-admin-binding", clusterRoleBindingDetail.Name)
	assert.Equal(t, "ClusterRole/cluster-admin", clusterRoleBindingDetail.RoleRef)
	require.Len(t, clusterRoleBindingDetail.Subjects, 2)
	assert.Equal(t, "ServiceAccount", clusterRoleBindingDetail.Subjects[0].Kind)
	assert.Equal(t, "marvin", clusterRoleBindingDetail.Subjects[0].Name)
	assert.Equal(t, "default", clusterRoleBindingDetail.Subjects[0].Namespace)
	assert.Equal(t, "User", clusterRoleBindingDetail.Subjects[1].Kind)
	assert.Equal(t, "alice", clusterRoleBindingDetail.Subjects[1].Name)
}

func TestClusterRoleBindingGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &clusterRoleBindingGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"name": "cluster-admin-binding"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestClusterRoleBindingGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &clusterRoleBindingGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestClusterRoleBindingGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &clusterRoleBindingGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
