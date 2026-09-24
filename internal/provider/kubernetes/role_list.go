package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*roleListTask)(nil)

type roleListTask struct{ provider *Provider }

func (t *roleListTask) Name() string { return "kubernetes.role.list" }

func (t *roleListTask) JSONSchema() string { return roleListSchema }

func (t *roleListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.RbacV1().Roles(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []roleSummary
	for _, role := range list.Items {
		items = append(items, flattenRole(role))
	}

	return common.SuccessResult(map[string]any{"roles": items, "count": len(items)}), nil
}

func flattenRole(role rbacv1.Role) roleSummary {
	return roleSummary{
		Name:       role.Name,
		Namespace:  role.Namespace,
		RulesCount: len(role.Rules),
		Age:        formatAge(role.CreationTimestamp.Time),
	}
}

const roleListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Role List Parameters",
  "description": "Parameters for listing Roles.",
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

type roleSummary struct {
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	RulesCount int    `json:"rules_count,omitempty"`
	Age        string `json:"age,omitempty"`
}
