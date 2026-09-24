package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*replicaSetGetTask)(nil)

type replicaSetGetTask struct{ provider *Provider }

func (t *replicaSetGetTask) Name() string { return "kubernetes.replicaset.get" }

func (t *replicaSetGetTask) JSONSchema() string { return replicaSetGetSchema }

func (t *replicaSetGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	rs, err := client.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"replicaset": flattenReplicaSetDetail(*rs)}), nil
}

func flattenReplicaSetDetail(rs appsv1.ReplicaSet) replicaSetDetail {
	owner := ""
	if len(rs.OwnerReferences) > 0 {
		owner = rs.OwnerReferences[0].Kind + "/" + rs.OwnerReferences[0].Name
	}
	return replicaSetDetail{
		Name:                 rs.Name,
		Namespace:            rs.Namespace,
		Replicas:             int(rs.Status.Replicas),
		ReadyReplicas:        int(rs.Status.ReadyReplicas),
		FullyLabeledReplicas: int(rs.Status.FullyLabeledReplicas),
		Age:                  formatAge(rs.CreationTimestamp.Time),
		OwnerReference:       owner,
		Labels:               rs.Labels,
	}
}

const replicaSetGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ReplicaSet Get Parameters",
  "description": "Parameters for retrieving a specific replicaset.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the replicaset."
    },
    "name": {
      "type": "string",
      "description": "Name of the replicaset."
    }
  },
  "required": ["namespace", "name"]
}`

type replicaSetDetail struct {
	Name                 string            `json:"name,omitempty"`
	Namespace            string            `json:"namespace,omitempty"`
	Replicas             int               `json:"replicas,omitempty"`
	ReadyReplicas        int               `json:"ready_replicas,omitempty"`
	FullyLabeledReplicas int               `json:"fully_labeled_replicas,omitempty"`
	Age                  string            `json:"age,omitempty"`
	OwnerReference       string            `json:"owner_reference,omitempty"`
	Labels               map[string]string `json:"labels,omitempty"`
}
