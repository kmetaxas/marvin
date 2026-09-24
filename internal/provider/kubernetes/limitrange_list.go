package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*limitRangeListTask)(nil)

type limitRangeListTask struct{ provider *Provider }

func (t *limitRangeListTask) Name() string { return "kubernetes.limitrange.list" }

func (t *limitRangeListTask) JSONSchema() string { return limitRangeListSchema }

func (t *limitRangeListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.CoreV1().LimitRanges(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []limitRangeSummary
	for _, lr := range list.Items {
		items = append(items, flattenLimitRange(lr))
	}

	return common.SuccessResult(map[string]any{"limit_ranges": items, "count": len(items)}), nil
}

func flattenLimitRange(lr corev1.LimitRange) limitRangeSummary {
	return limitRangeSummary{
		Name:      lr.Name,
		Namespace: lr.Namespace,
		Age:       formatAge(lr.CreationTimestamp.Time),
	}
}

const limitRangeListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "LimitRange List Parameters",
  "description": "Parameters for listing LimitRanges.",
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

type limitRangeSummary struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Age       string `json:"age,omitempty"`
}
