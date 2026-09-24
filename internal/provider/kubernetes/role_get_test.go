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

func TestRoleGetTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider *Provider
		params   map[string]any
		assert   func(*testing.T, any, string)
	}{
		{
			name: "success",
			provider: newTestProvider(&k8sClient{
				clientset: fake.NewSimpleClientset(&rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-reader",
						Namespace: "default",
					},
					Rules: []rbacv1.PolicyRule{{
						Verbs:         []string{"watch", "get", "list"},
						APIGroups:     []string{""},
						Resources:     []string{"pods"},
						ResourceNames: []string{"pod-a"},
					}},
				},
				),
			}),
			params: map[string]any{"namespace": "default", "name": "pod-reader"},
			assert: func(t *testing.T, data any, taskError string) {
				t.Helper()
				assert.Empty(t, taskError)
				payload := data.(map[string]any)
				role := payload["role"].(roleDetail)
				assert.Equal(t, "pod-reader", role.Name)
				assert.Equal(t, "default", role.Namespace)
				require.Len(t, role.Rules, 1)
				assert.Equal(t, []string{"get", "list", "watch"}, role.Rules[0].Verbs)
				assert.Equal(t, []string{""}, role.Rules[0].APIGroups)
				assert.Equal(t, []string{"pods"}, role.Rules[0].Resources)
				assert.Equal(t, []string{"pod-a"}, role.Rules[0].ResourceNames)
			},
		},
		{
			name:     "client not configured",
			provider: newTestProvider(&k8sClient{}),
			params:   map[string]any{"namespace": "default", "name": "pod-reader"},
			assert: func(t *testing.T, data any, taskError string) {
				t.Helper()
				assert.Nil(t, data)
				assert.Contains(t, taskError, "kubernetes client is not configured")
			},
		},
		{
			name:     "missing params",
			provider: newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()}),
			params:   map[string]any{"namespace": "default"},
			assert: func(t *testing.T, data any, taskError string) {
				t.Helper()
				assert.Nil(t, data)
				assert.Contains(t, taskError, "name")
			},
		},
		{
			name:     "not found",
			provider: newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()}),
			params:   map[string]any{"namespace": "default", "name": "missing"},
			assert: func(t *testing.T, data any, taskError string) {
				t.Helper()
				assert.Nil(t, data)
				assert.NotEmpty(t, taskError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &roleGetTask{provider: tt.provider}
			res, err := task.Execute(context.Background(), tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.name == "success", res.Success)
			tt.assert(t, res.Data, res.Error)
		})
	}
}
