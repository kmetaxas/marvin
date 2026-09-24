package kubernetes

import (
	"context"
	"sort"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*roleGetTask)(nil)

type roleGetTask struct{ provider *Provider }

func (t *roleGetTask) Name() string { return "kubernetes.role.get" }

func (t *roleGetTask) JSONSchema() string { return roleGetSchema }

func (t *roleGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	role, err := client.clientset.RbacV1().Roles(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"role": flattenRoleDetail(*role)}), nil
}

func flattenRoleDetail(role rbacv1.Role) roleDetail {
	rules := make([]ruleDetail, 0, len(role.Rules))
	for _, rule := range role.Rules {
		rules = append(rules, flattenPolicyRule(rule))
	}

	return roleDetail{
		Name:      role.Name,
		Namespace: role.Namespace,
		Rules:     rules,
		Age:       formatAge(role.CreationTimestamp.Time),
	}
}

func flattenPolicyRule(rule rbacv1.PolicyRule) ruleDetail {
	return ruleDetail{
		Verbs:         sortedStringSlice(rule.Verbs),
		APIGroups:     sortedStringSlice(rule.APIGroups),
		Resources:     sortedStringSlice(rule.Resources),
		ResourceNames: sortedStringSlice(rule.ResourceNames),
	}
}

func sortedStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}

	cloned := append([]string(nil), items...)
	sort.Strings(cloned)
	return cloned
}

const roleGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Role Get Parameters",
  "description": "Parameters for retrieving a specific Role.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the Role."
    },
    "name": {
      "type": "string",
      "description": "Name of the Role."
    }
  },
  "required": ["namespace", "name"]
}`

type roleDetail struct {
	Name      string       `json:"name,omitempty"`
	Namespace string       `json:"namespace,omitempty"`
	Rules     []ruleDetail `json:"rules,omitempty"`
	Age       string       `json:"age,omitempty"`
}

type ruleDetail struct {
	Verbs         []string `json:"verbs,omitempty"`
	APIGroups     []string `json:"api_groups,omitempty"`
	Resources     []string `json:"resources,omitempty"`
	ResourceNames []string `json:"resource_names,omitempty"`
}
