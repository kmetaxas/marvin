package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*roleBindingGetTask)(nil)

type roleBindingGetTask struct{ provider *Provider }

func (t *roleBindingGetTask) Name() string { return "kubernetes.rolebinding.get" }

func (t *roleBindingGetTask) JSONSchema() string { return roleBindingGetSchema }

func (t *roleBindingGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.RequireString(params, "namespace")
	if err != nil {
		return common.TaskFailure(err)
	}
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	roleBinding, err := client.clientset.RbacV1().RoleBindings(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"role_binding": flattenRoleBindingDetail(*roleBinding)}), nil
}

func flattenRoleBindingDetail(roleBinding rbacv1.RoleBinding) roleBindingDetail {
	subjects := make([]subjectDetail, 0, len(roleBinding.Subjects))
	for _, subject := range roleBinding.Subjects {
		subjects = append(subjects, subjectDetail{
			Kind:      subject.Kind,
			Name:      subject.Name,
			Namespace: subject.Namespace,
		})
	}

	return roleBindingDetail{
		Name:      roleBinding.Name,
		Namespace: roleBinding.Namespace,
		RoleRef:   formatRoleRef(roleBinding.RoleRef),
		Subjects:  subjects,
		Age:       formatAge(roleBinding.CreationTimestamp.Time),
	}
}

const roleBindingGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "RoleBinding Get Parameters",
  "description": "Parameters for retrieving a specific RoleBinding.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the RoleBinding."
    },
    "name": {
      "type": "string",
      "description": "Name of the RoleBinding."
    }
  },
  "required": ["namespace", "name"]
}`

type roleBindingDetail struct {
	Name      string          `json:"name,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
	RoleRef   string          `json:"role_ref,omitempty"`
	Subjects  []subjectDetail `json:"subjects,omitempty"`
	Age       string          `json:"age,omitempty"`
}

type subjectDetail struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}
