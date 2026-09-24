package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*roleBindingListTask)(nil)

type roleBindingListTask struct{ provider *Provider }

func (t *roleBindingListTask) Name() string { return "kubernetes.rolebinding.list" }

func (t *roleBindingListTask) JSONSchema() string { return roleBindingListSchema }

func (t *roleBindingListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.OptionalString(params, "namespace", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	labelSelector, err := common.OptionalString(params, "label_selector", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	limit, err := common.NormalizeLimit(params)
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		Limit:         int64(limit),
	}

	list, err := client.clientset.RbacV1().RoleBindings(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []roleBindingSummary
	for _, roleBinding := range list.Items {
		items = append(items, flattenRoleBinding(roleBinding))
	}

	return common.SuccessResult(map[string]any{"role_bindings": items, "count": len(items)}), nil
}

func flattenRoleBinding(roleBinding rbacv1.RoleBinding) roleBindingSummary {
	return roleBindingSummary{
		Name:          roleBinding.Name,
		Namespace:     roleBinding.Namespace,
		RoleRef:       formatRoleRef(roleBinding.RoleRef),
		SubjectsCount: len(roleBinding.Subjects),
		Age:           formatAge(roleBinding.CreationTimestamp.Time),
	}
}

func formatRoleRef(roleRef rbacv1.RoleRef) string {
	return roleRef.Kind + "/" + roleRef.Name
}

const roleBindingListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "RoleBinding List Parameters",
  "description": "Parameters for listing RoleBindings.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit to list across all namespaces."
    },
    "label_selector": {
      "type": "string",
      "description": "Label selector expression."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of items to return.",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  }
}`

type roleBindingSummary struct {
	Name          string `json:"name,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	RoleRef       string `json:"role_ref,omitempty"`
	SubjectsCount int    `json:"subjects_count,omitempty"`
	Age           string `json:"age,omitempty"`
}
