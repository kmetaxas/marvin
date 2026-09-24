package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statefulSetListTask)(nil)

type statefulSetListTask struct{ provider *Provider }

func (t *statefulSetListTask) Name() string { return "kubernetes.statefulset.list" }

func (t *statefulSetListTask) JSONSchema() string { return statefulSetListSchema }

func (t *statefulSetListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.OptionalString(params, "namespace", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	labelSelector, err := common.OptionalString(params, "label_selector", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	fieldSelector, err := common.OptionalString(params, "field_selector", "")
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
		FieldSelector: fieldSelector,
		Limit:         int64(limit),
	}

	list, err := client.clientset.AppsV1().StatefulSets(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []statefulSetSummary
	for _, ss := range list.Items {
		items = append(items, flattenStatefulSet(ss))
	}

	return common.SuccessResult(map[string]any{"statefulsets": items, "count": len(items)}), nil
}

func flattenStatefulSet(ss appsv1.StatefulSet) statefulSetSummary {
	return statefulSetSummary{
		Name:            ss.Name,
		Namespace:       ss.Namespace,
		Replicas:        int(ss.Status.Replicas),
		ReadyReplicas:   int(ss.Status.ReadyReplicas),
		UpdatedReplicas: int(ss.Status.UpdatedReplicas),
		Age:             formatAge(ss.CreationTimestamp.Time),
		ServiceName:     ss.Spec.ServiceName,
	}
}

const statefulSetListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "StatefulSet List Parameters",
  "description": "Parameters for listing statefulsets.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit to list across all namespaces."
    },
    "label_selector": {
      "type": "string",
      "description": "Label selector expression (e.g., 'app=nginx')."
    },
    "field_selector": {
      "type": "string",
      "description": "Field selector expression (e.g., 'status.phase=Running')."
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

type statefulSetSummary struct {
	Name            string `json:"name,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	Replicas        int    `json:"replicas,omitempty"`
	ReadyReplicas   int    `json:"ready_replicas,omitempty"`
	UpdatedReplicas int    `json:"updated_replicas,omitempty"`
	Age             string `json:"age,omitempty"`
	ServiceName     string `json:"service_name,omitempty"`
}
