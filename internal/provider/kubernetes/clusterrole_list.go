package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*clusterRoleListTask)(nil)

type clusterRoleListTask struct{ provider *Provider }

func (t *clusterRoleListTask) Name() string { return "kubernetes.clusterrole.list" }

func (t *clusterRoleListTask) JSONSchema() string { return clusterRoleListSchema }

func (t *clusterRoleListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.RbacV1().ClusterRoles().List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []clusterRoleSummary
	for _, clusterRole := range list.Items {
		items = append(items, flattenClusterRole(clusterRole))
	}

	return common.SuccessResult(map[string]any{"cluster_roles": items, "count": len(items)}), nil
}

func flattenClusterRole(clusterRole rbacv1.ClusterRole) clusterRoleSummary {
	return clusterRoleSummary{
		Name:       clusterRole.Name,
		RulesCount: len(clusterRole.Rules),
		Age:        formatAge(clusterRole.CreationTimestamp.Time),
	}
}

const clusterRoleListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ClusterRole List Parameters",
  "description": "Parameters for listing ClusterRoles.",
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

type clusterRoleSummary struct {
	Name       string `json:"name,omitempty"`
	RulesCount int    `json:"rules_count,omitempty"`
	Age        string `json:"age,omitempty"`
}
