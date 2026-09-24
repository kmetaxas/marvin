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

func TestRoleBindingListTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider *Provider
		params   map[string]any
		assert   func(*testing.T, any, string)
	}{
		{
			name: "success",
			provider: newTestProvider(
				&k8sClient{
					clientset: fake.NewSimpleClientset(
						&rbacv1.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "read-pods",
								Namespace: "default",
							},
							RoleRef:  rbacv1.RoleRef{Kind: "Role", Name: "pod-reader", APIGroup: "rbac.authorization.k8s.io"},
							Subjects: []rbacv1.Subject{{Kind: "ServiceAccount", Name: "default", Namespace: "default"}},
						},
					),
				},
			),
			params: map[string]any{"namespace": "default"},
			assert: func(t *testing.T, data any, taskError string) {
				t.Helper()
				assert.Empty(t, taskError)
				payload := data.(map[string]any)
				assert.Equal(t, 1, payload["count"])
				bindings := payload["role_bindings"].([]roleBindingSummary)
				require.Len(t, bindings, 1)
				assert.Equal(t, "read-pods", bindings[0].Name)
				assert.Equal(t, "default", bindings[0].Namespace)
				assert.Equal(t, "Role/pod-reader", bindings[0].RoleRef)
				assert.Equal(t, 1, bindings[0].SubjectsCount)
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
			task := &roleBindingListTask{provider: tt.provider}
			res, err := task.Execute(context.Background(), tt.params)
			require.NoError(t, err)
			assert.Equal(t, tt.name == "success", res.Success)
			tt.assert(t, res.Data, res.Error)
		})
	}
}
