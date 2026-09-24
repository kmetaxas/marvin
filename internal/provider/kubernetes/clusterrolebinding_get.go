package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*clusterRoleBindingGetTask)(nil)

type clusterRoleBindingGetTask struct{ provider *Provider }

func (t *clusterRoleBindingGetTask) Name() string { return "kubernetes.clusterrolebinding.get" }

func (t *clusterRoleBindingGetTask) JSONSchema() string { return clusterRoleBindingGetSchema }

func (t *clusterRoleBindingGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	clusterRoleBinding, err := client.clientset.RbacV1().ClusterRoleBindings().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"cluster_role_binding": flattenClusterRoleBindingDetail(*clusterRoleBinding)}), nil
}

func flattenClusterRoleBindingDetail(clusterRoleBinding rbacv1.ClusterRoleBinding) clusterRoleBindingDetail {
	subjects := make([]subjectDetail, 0, len(clusterRoleBinding.Subjects))
	for _, subject := range clusterRoleBinding.Subjects {
		subjects = append(subjects, subjectDetail{
			Kind:      subject.Kind,
			Name:      subject.Name,
			Namespace: subject.Namespace,
		})
	}

	return clusterRoleBindingDetail{
		Name:     clusterRoleBinding.Name,
		RoleRef:  formatRoleRef(clusterRoleBinding.RoleRef),
		Subjects: subjects,
		Age:      formatAge(clusterRoleBinding.CreationTimestamp.Time),
	}
}

const clusterRoleBindingGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ClusterRoleBinding Get Parameters",
  "description": "Parameters for retrieving a specific ClusterRoleBinding.",
  "properties": {
    "name": {
      "type": "string",
      "description": "Name of the ClusterRoleBinding."
    }
  },
  "required": ["name"]
}`

type clusterRoleBindingDetail struct {
	Name     string          `json:"name,omitempty"`
	RoleRef  string          `json:"role_ref,omitempty"`
	Subjects []subjectDetail `json:"subjects,omitempty"`
	Age      string          `json:"age,omitempty"`
}
