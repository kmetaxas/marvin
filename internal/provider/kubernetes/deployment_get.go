package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*deploymentGetTask)(nil)

type deploymentGetTask struct{ provider *Provider }

func (t *deploymentGetTask) Name() string { return "kubernetes.deployment.get" }

func (t *deploymentGetTask) JSONSchema() string { return deploymentGetSchema }

func (t *deploymentGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	dep, err := client.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"deployment": flattenDeploymentDetail(*dep)}), nil
}

func flattenDeploymentDetail(dep appsv1.Deployment) deploymentDetail {
	var conditions []conditionSummary
	for _, c := range dep.Status.Conditions {
		conditions = append(conditions, conditionSummary{
			Type:   string(c.Type),
			Status: string(c.Status),
			Reason: c.Reason,
		})
	}
	return deploymentDetail{
		Name:              dep.Name,
		Namespace:         dep.Namespace,
		Replicas:          int(dep.Status.Replicas),
		ReadyReplicas:     int(dep.Status.ReadyReplicas),
		UpdatedReplicas:   int(dep.Status.UpdatedReplicas),
		AvailableReplicas: int(dep.Status.AvailableReplicas),
		Strategy:          string(dep.Spec.Strategy.Type),
		Selector:          dep.Spec.Selector.MatchLabels,
		Conditions:        conditions,
		Age:               formatAge(dep.CreationTimestamp.Time),
		Labels:            dep.Labels,
	}
}

const deploymentGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Deployment Get Parameters",
  "description": "Parameters for retrieving a specific deployment.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the deployment."
    },
    "name": {
      "type": "string",
      "description": "Name of the deployment."
    }
  },
  "required": ["namespace", "name"]
}`

type deploymentDetail struct {
	Name              string             `json:"name,omitempty"`
	Namespace         string             `json:"namespace,omitempty"`
	Replicas          int                `json:"replicas,omitempty"`
	ReadyReplicas     int                `json:"ready_replicas,omitempty"`
	UpdatedReplicas   int                `json:"updated_replicas,omitempty"`
	AvailableReplicas int                `json:"available_replicas,omitempty"`
	Strategy          string             `json:"strategy,omitempty"`
	Selector          map[string]string  `json:"selector,omitempty"`
	Conditions        []conditionSummary `json:"conditions,omitempty"`
	Age               string             `json:"age,omitempty"`
	Labels            map[string]string  `json:"labels,omitempty"`
}
