package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*clusterRoleGetTask)(nil)

type clusterRoleGetTask struct{ provider *Provider }

func (t *clusterRoleGetTask) Name() string { return "kubernetes.clusterrole.get" }

func (t *clusterRoleGetTask) JSONSchema() string { return clusterRoleGetSchema }

func (t *clusterRoleGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	clusterRole, err := client.clientset.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"cluster_role": flattenClusterRoleDetail(*clusterRole)}), nil
}

func flattenClusterRoleDetail(clusterRole rbacv1.ClusterRole) clusterRoleDetail {
	rules := make([]ruleDetail, 0, len(clusterRole.Rules))
	for _, rule := range clusterRole.Rules {
		rules = append(rules, flattenPolicyRule(rule))
	}

	return clusterRoleDetail{
		Name:  clusterRole.Name,
		Rules: rules,
		Age:   formatAge(clusterRole.CreationTimestamp.Time),
	}
}

const clusterRoleGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ClusterRole Get Parameters",
  "description": "Parameters for retrieving a specific ClusterRole.",
  "properties": {
    "name": {
      "type": "string",
      "description": "Name of the ClusterRole."
    }
  },
  "required": ["name"]
}`

type clusterRoleDetail struct {
	Name  string       `json:"name,omitempty"`
	Rules []ruleDetail `json:"rules,omitempty"`
	Age   string       `json:"age,omitempty"`
}
