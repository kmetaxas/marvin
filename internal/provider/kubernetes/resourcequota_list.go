package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*resourceQuotaListTask)(nil)

type resourceQuotaListTask struct{ provider *Provider }

func (t *resourceQuotaListTask) Name() string { return "kubernetes.resourcequota.list" }

func (t *resourceQuotaListTask) JSONSchema() string { return resourceQuotaListSchema }

func (t *resourceQuotaListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.CoreV1().ResourceQuotas(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []resourceQuotaSummary
	for _, rq := range list.Items {
		items = append(items, flattenResourceQuota(rq))
	}

	return common.SuccessResult(map[string]any{"resource_quotas": items, "count": len(items)}), nil
}

func flattenResourceQuota(rq corev1.ResourceQuota) resourceQuotaSummary {
	return resourceQuotaSummary{
		Name:      rq.Name,
		Namespace: rq.Namespace,
		Age:       formatAge(rq.CreationTimestamp.Time),
	}
}

const resourceQuotaListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ResourceQuota List Parameters",
  "description": "Parameters for listing ResourceQuotas.",
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

type resourceQuotaSummary struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Age       string `json:"age,omitempty"`
}
