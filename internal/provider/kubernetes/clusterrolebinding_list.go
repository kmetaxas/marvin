package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*clusterRoleBindingListTask)(nil)

type clusterRoleBindingListTask struct{ provider *Provider }

func (t *clusterRoleBindingListTask) Name() string { return "kubernetes.clusterrolebinding.list" }

func (t *clusterRoleBindingListTask) JSONSchema() string { return clusterRoleBindingListSchema }

func (t *clusterRoleBindingListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.RbacV1().ClusterRoleBindings().List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []clusterRoleBindingSummary
	for _, clusterRoleBinding := range list.Items {
		items = append(items, flattenClusterRoleBinding(clusterRoleBinding))
	}

	return common.SuccessResult(map[string]any{"cluster_role_bindings": items, "count": len(items)}), nil
}

func flattenClusterRoleBinding(clusterRoleBinding rbacv1.ClusterRoleBinding) clusterRoleBindingSummary {
	return clusterRoleBindingSummary{
		Name:          clusterRoleBinding.Name,
		RoleRef:       formatRoleRef(clusterRoleBinding.RoleRef),
		SubjectsCount: len(clusterRoleBinding.Subjects),
		Age:           formatAge(clusterRoleBinding.CreationTimestamp.Time),
	}
}

const clusterRoleBindingListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ClusterRoleBinding List Parameters",
  "description": "Parameters for listing ClusterRoleBindings.",
  "properties": {
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

type clusterRoleBindingSummary struct {
	Name          string `json:"name,omitempty"`
	RoleRef       string `json:"role_ref,omitempty"`
	SubjectsCount int    `json:"subjects_count,omitempty"`
	Age           string `json:"age,omitempty"`
}
