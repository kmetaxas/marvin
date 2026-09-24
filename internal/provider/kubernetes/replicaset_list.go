package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*replicaSetListTask)(nil)

type replicaSetListTask struct{ provider *Provider }

func (t *replicaSetListTask) Name() string { return "kubernetes.replicaset.list" }

func (t *replicaSetListTask) JSONSchema() string { return replicaSetListSchema }

func (t *replicaSetListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.AppsV1().ReplicaSets(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []replicaSetSummary
	for _, rs := range list.Items {
		items = append(items, flattenReplicaSet(rs))
	}

	return common.SuccessResult(map[string]any{"replicasets": items, "count": len(items)}), nil
}

func flattenReplicaSet(rs appsv1.ReplicaSet) replicaSetSummary {
	owner := ""
	if len(rs.OwnerReferences) > 0 {
		owner = rs.OwnerReferences[0].Kind + "/" + rs.OwnerReferences[0].Name
	}
	return replicaSetSummary{
		Name:                 rs.Name,
		Namespace:            rs.Namespace,
		Replicas:             int(rs.Status.Replicas),
		ReadyReplicas:        int(rs.Status.ReadyReplicas),
		FullyLabeledReplicas: int(rs.Status.FullyLabeledReplicas),
		Age:                  formatAge(rs.CreationTimestamp.Time),
		OwnerReference:       owner,
	}
}

const replicaSetListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ReplicaSet List Parameters",
  "description": "Parameters for listing replicasets.",
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

type replicaSetSummary struct {
	Name                 string `json:"name,omitempty"`
	Namespace            string `json:"namespace,omitempty"`
	Replicas             int    `json:"replicas,omitempty"`
	ReadyReplicas        int    `json:"ready_replicas,omitempty"`
	FullyLabeledReplicas int    `json:"fully_labeled_replicas,omitempty"`
	Age                  string `json:"age,omitempty"`
	OwnerReference       string `json:"owner_reference,omitempty"`
}
