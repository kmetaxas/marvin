package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*limitRangeGetTask)(nil)

type limitRangeGetTask struct{ provider *Provider }

func (t *limitRangeGetTask) Name() string { return "kubernetes.limitrange.get" }

func (t *limitRangeGetTask) JSONSchema() string { return limitRangeGetSchema }

func (t *limitRangeGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	lr, err := client.clientset.CoreV1().LimitRanges(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"limit_range": flattenLimitRangeDetail(*lr)}), nil
}

func flattenLimitRangeDetail(lr corev1.LimitRange) limitRangeDetail {
	var limits []limitRangeItemDetail
	for _, spec := range lr.Spec.Limits {
		limits = append(limits, limitRangeItemDetail{
			Type:           string(spec.Type),
			Default:        flattenResourceList(spec.Default),
			DefaultRequest: flattenResourceList(spec.DefaultRequest),
			Max:            flattenResourceList(spec.Max),
			Min:            flattenResourceList(spec.Min),
		})
	}
	return limitRangeDetail{
		Name:      lr.Name,
		Namespace: lr.Namespace,
		Limits:    limits,
		Age:       formatAge(lr.CreationTimestamp.Time),
	}
}

const limitRangeGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "LimitRange Get Parameters",
  "description": "Parameters for retrieving a specific LimitRange.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the LimitRange."
    },
    "name": {
      "type": "string",
      "description": "Name of the LimitRange."
    }
  },
  "required": ["namespace", "name"]
}`

type limitRangeDetail struct {
	Name      string                 `json:"name,omitempty"`
	Namespace string                 `json:"namespace,omitempty"`
	Limits    []limitRangeItemDetail `json:"limits,omitempty"`
	Age       string                 `json:"age,omitempty"`
}

type limitRangeItemDetail struct {
	Type           string            `json:"type,omitempty"`
	Default        map[string]string `json:"default,omitempty"`
	DefaultRequest map[string]string `json:"default_request,omitempty"`
	Max            map[string]string `json:"max,omitempty"`
	Min            map[string]string `json:"min,omitempty"`
}
