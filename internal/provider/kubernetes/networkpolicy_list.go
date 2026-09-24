package kubernetes

import (
	"context"
	"maps"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*networkPolicyListTask)(nil)

type networkPolicyListTask struct{ provider *Provider }

func (t *networkPolicyListTask) Name() string { return "kubernetes.networkpolicy.list" }

func (t *networkPolicyListTask) JSONSchema() string { return networkPolicyListSchema }

func (t *networkPolicyListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []networkPolicySummary
	for _, networkPolicy := range list.Items {
		items = append(items, flattenNetworkPolicy(networkPolicy))
	}

	return common.SuccessResult(map[string]any{"network_policies": items, "count": len(items)}), nil
}

func flattenNetworkPolicy(networkPolicy networkingv1.NetworkPolicy) networkPolicySummary {
	return networkPolicySummary{
		Name:              networkPolicy.Name,
		Namespace:         networkPolicy.Namespace,
		PodSelector:       matchLabelsOrEmpty(networkPolicy.Spec.PodSelector),
		PolicyTypes:       flattenPolicyTypes(networkPolicy.Spec.PolicyTypes),
		IngressRulesCount: len(networkPolicy.Spec.Ingress),
		EgressRulesCount:  len(networkPolicy.Spec.Egress),
		Age:               formatAge(networkPolicy.CreationTimestamp.Time),
	}
}

func flattenPolicyTypes(policyTypes []networkingv1.PolicyType) []string {
	items := make([]string, 0, len(policyTypes))
	for _, policyType := range policyTypes {
		items = append(items, string(policyType))
	}
	return items
}

func matchLabelsOrEmpty(selector metav1.LabelSelector) map[string]string {
	if len(selector.MatchLabels) == 0 {
		return map[string]string{}
	}
	labels := make(map[string]string, len(selector.MatchLabels))
	maps.Copy(labels, selector.MatchLabels)
	return labels
}

const networkPolicyListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "NetworkPolicy List Parameters",
  "description": "Parameters for listing NetworkPolicies.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit to list across all namespaces."
    },
    "label_selector": {
      "type": "string",
      "description": "Label selector expression (e.g., 'app=nginx')."
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

type networkPolicySummary struct {
	Name              string            `json:"name,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	PodSelector       map[string]string `json:"pod_selector,omitempty"`
	PolicyTypes       []string          `json:"policy_types,omitempty"`
	IngressRulesCount int               `json:"ingress_rules_count,omitempty"`
	EgressRulesCount  int               `json:"egress_rules_count,omitempty"`
	Age               string            `json:"age,omitempty"`
}
