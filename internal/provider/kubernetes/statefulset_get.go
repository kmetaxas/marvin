package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*statefulSetGetTask)(nil)

type statefulSetGetTask struct{ provider *Provider }

func (t *statefulSetGetTask) Name() string { return "kubernetes.statefulset.get" }

func (t *statefulSetGetTask) JSONSchema() string { return statefulSetGetSchema }

func (t *statefulSetGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	ss, err := client.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"statefulset": flattenStatefulSetDetail(*ss)}), nil
}

func flattenStatefulSetDetail(ss appsv1.StatefulSet) statefulSetDetail {
	return statefulSetDetail{
		Name:            ss.Name,
		Namespace:       ss.Namespace,
		Replicas:        int(ss.Status.Replicas),
		ReadyReplicas:   int(ss.Status.ReadyReplicas),
		UpdatedReplicas: int(ss.Status.UpdatedReplicas),
		Age:             formatAge(ss.CreationTimestamp.Time),
		ServiceName:     ss.Spec.ServiceName,
		Labels:          ss.Labels,
	}
}

const statefulSetGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "StatefulSet Get Parameters",
  "description": "Parameters for retrieving a specific statefulset.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the statefulset."
    },
    "name": {
      "type": "string",
      "description": "Name of the statefulset."
    }
  },
  "required": ["namespace", "name"]
}`

type statefulSetDetail struct {
	Name            string            `json:"name,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	Replicas        int               `json:"replicas,omitempty"`
	ReadyReplicas   int               `json:"ready_replicas,omitempty"`
	UpdatedReplicas int               `json:"updated_replicas,omitempty"`
	Age             string            `json:"age,omitempty"`
	ServiceName     string            `json:"service_name,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
}
