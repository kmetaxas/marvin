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

func TestClusterRoleBindingListTask(t *testing.T) {
	t.Parallel()

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-admin-binding",
		},
		RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "marvin", Namespace: "default"},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(clusterRoleBinding)})
	task := &clusterRoleBindingListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	clusterRoleBindings := data["cluster_role_bindings"].([]clusterRoleBindingSummary)
	require.Len(t, clusterRoleBindings, 1)
	assert.Equal(t, "cluster-admin-binding", clusterRoleBindings[0].Name)
	assert.Equal(t, "ClusterRole/cluster-admin", clusterRoleBindings[0].RoleRef)
	assert.Equal(t, 1, clusterRoleBindings[0].SubjectsCount)
}

func TestClusterRoleBindingListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &clusterRoleBindingListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}
