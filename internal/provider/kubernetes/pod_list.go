package kubernetes

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*podListTask)(nil)

type podListTask struct{ provider *Provider }

func (t *podListTask) Name() string { return "kubernetes.pod.list" }

func (t *podListTask) JSONSchema() string { return podListSchema }

func (t *podListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.CoreV1().Pods(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []podSummary
	for _, pod := range list.Items {
		items = append(items, flattenPod(pod))
	}

	return common.SuccessResult(map[string]any{"pods": items, "count": len(items)}), nil
}

func flattenPod(pod corev1.Pod) podSummary {
	ready := 0
	restarts := 0
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
		restarts += int(cs.RestartCount)
	}
	return podSummary{
		Name:            pod.Name,
		Namespace:       pod.Namespace,
		Status:          string(pod.Status.Phase),
		ReadyContainers: fmt.Sprintf("%d/%d", ready, len(pod.Status.ContainerStatuses)),
		Restarts:        restarts,
		Age:             formatAge(pod.CreationTimestamp.Time),
		Labels:          pod.Labels,
		NodeName:        pod.Spec.NodeName,
	}
}

const podListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Pod List Parameters",
  "description": "Parameters for listing pods.",
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

type podSummary struct {
	Name            string            `json:"name,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	Status          string            `json:"status,omitempty"`
	ReadyContainers string            `json:"ready_containers,omitempty"`
	Restarts        int               `json:"restarts,omitempty"`
	Age             string            `json:"age,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	NodeName        string            `json:"node_name,omitempty"`
}
