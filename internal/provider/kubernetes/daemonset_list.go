package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*daemonSetListTask)(nil)

type daemonSetListTask struct{ provider *Provider }

func (t *daemonSetListTask) Name() string { return "kubernetes.daemonset.list" }

func (t *daemonSetListTask) JSONSchema() string { return daemonSetListSchema }

func (t *daemonSetListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.AppsV1().DaemonSets(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []daemonSetSummary
	for _, ds := range list.Items {
		items = append(items, flattenDaemonSet(ds))
	}

	return common.SuccessResult(map[string]any{"daemonsets": items, "count": len(items)}), nil
}

func flattenDaemonSet(ds appsv1.DaemonSet) daemonSetSummary {
	return daemonSetSummary{
		Name:      ds.Name,
		Namespace: ds.Namespace,
		Desired:   int(ds.Status.DesiredNumberScheduled),
		Current:   int(ds.Status.CurrentNumberScheduled),
		Ready:     int(ds.Status.NumberReady),
		Updated:   int(ds.Status.UpdatedNumberScheduled),
		Available: int(ds.Status.NumberAvailable),
		Age:       formatAge(ds.CreationTimestamp.Time),
	}
}

const daemonSetListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "DaemonSet List Parameters",
  "description": "Parameters for listing daemonsets.",
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

type daemonSetSummary struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Desired   int    `json:"desired,omitempty"`
	Current   int    `json:"current,omitempty"`
	Ready     int    `json:"ready,omitempty"`
	Updated   int    `json:"updated,omitempty"`
	Available int    `json:"available,omitempty"`
	Age       string `json:"age,omitempty"`
}
