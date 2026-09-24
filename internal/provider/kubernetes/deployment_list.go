package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*deploymentListTask)(nil)

type deploymentListTask struct{ provider *Provider }

func (t *deploymentListTask) Name() string { return "kubernetes.deployment.list" }

func (t *deploymentListTask) JSONSchema() string { return deploymentListSchema }

func (t *deploymentListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.AppsV1().Deployments(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []deploymentSummary
	for _, d := range list.Items {
		items = append(items, flattenDeployment(d))
	}

	return common.SuccessResult(map[string]any{"deployments": items, "count": len(items)}), nil
}

func flattenDeployment(d appsv1.Deployment) deploymentSummary {
	return deploymentSummary{
		Name:              d.Name,
		Namespace:         d.Namespace,
		Replicas:          int(d.Status.Replicas),
		ReadyReplicas:     int(d.Status.ReadyReplicas),
		UpdatedReplicas:   int(d.Status.UpdatedReplicas),
		AvailableReplicas: int(d.Status.AvailableReplicas),
		Strategy:          string(d.Spec.Strategy.Type),
		Age:               formatAge(d.CreationTimestamp.Time),
		Labels:            d.Labels,
	}
}

const deploymentListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Deployment List Parameters",
  "description": "Parameters for listing deployments.",
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

type deploymentSummary struct {
	Name              string            `json:"name,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	Replicas          int               `json:"replicas,omitempty"`
	ReadyReplicas     int               `json:"ready_replicas,omitempty"`
	UpdatedReplicas   int               `json:"updated_replicas,omitempty"`
	AvailableReplicas int               `json:"available_replicas,omitempty"`
	Strategy          string            `json:"strategy,omitempty"`
	Age               string            `json:"age,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}
