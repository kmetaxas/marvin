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

func TestClusterRoleGetTask(t *testing.T) {
	t.Parallel()

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-admin",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get", "list", "watch"},
				APIGroups:     []string{""},
				Resources:     []string{"pods"},
				ResourceNames: []string{"nginx"},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(clusterRole)})
	task := &clusterRoleGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "cluster-admin"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	clusterRoleDetail := data["cluster_role"].(clusterRoleDetail)
	assert.Equal(t, "cluster-admin", clusterRoleDetail.Name)
	require.Len(t, clusterRoleDetail.Rules, 1)
	assert.Equal(t, []string{"get", "list", "watch"}, clusterRoleDetail.Rules[0].Verbs)
	assert.Equal(t, []string{""}, clusterRoleDetail.Rules[0].APIGroups)
	assert.Equal(t, []string{"pods"}, clusterRoleDetail.Rules[0].Resources)
	assert.Equal(t, []string{"nginx"}, clusterRoleDetail.Rules[0].ResourceNames)
}

func TestClusterRoleGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &clusterRoleGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"name": "cluster-admin"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestClusterRoleGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &clusterRoleGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestClusterRoleGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &clusterRoleGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
