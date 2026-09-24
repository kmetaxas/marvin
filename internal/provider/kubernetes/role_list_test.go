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

func TestRoleListTask(t *testing.T) {
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
					Rules: []rbacv1.PolicyRule{{Verbs: []string{"get", "list", "watch"}, Resources: []string{"pods"}}},
				},
				),
			}),
			params: map[string]any{"namespace": "default"},
			assert: func(t *testing.T, data any, taskError string) {
				t.Helper()
				assert.Empty(t, taskError)
				payload := data.(map[string]any)
				assert.Equal(t, 1, payload["count"])
				roles := payload["roles"].([]roleSummary)
				require.Len(t, roles, 1)
				assert.Equal(t, "pod-reader", roles[0].Name)
				assert.Equal(t, "default", roles[0].Namespace)
				assert.Equal(t, 1, roles[0].RulesCount)
			},
		},
		{
			name:     "client not configured",
			provider: newTestProvider(&k8sClient{}),
			params:   map[string]any{},
			assert: func(t *testing.T, data any, taskError string) {
				t.Helper()
				assert.Nil(t, data)
				assert.Contains(t, taskError, "kubernetes client is not configured")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &roleListTask{provider: tt.provider}
			res, err := task.Execute(context.Background(), tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.name == "success", res.Success)
			tt.assert(t, res.Data, res.Error)
		})
	}
}
