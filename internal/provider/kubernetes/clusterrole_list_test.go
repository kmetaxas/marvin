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

func TestClusterRoleListTask(t *testing.T) {
	t.Parallel()

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-admin",
		},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get", "list", "watch"}, APIGroups: []string{""}, Resources: []string{"pods"}},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(clusterRole)})
	task := &clusterRoleListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	clusterRoles := data["cluster_roles"].([]clusterRoleSummary)
	require.Len(t, clusterRoles, 1)
	assert.Equal(t, "cluster-admin", clusterRoles[0].Name)
	assert.Equal(t, 1, clusterRoles[0].RulesCount)
}

func TestClusterRoleListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &clusterRoleListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}
