package kubernetes

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*daemonSetGetTask)(nil)

type daemonSetGetTask struct{ provider *Provider }

func (t *daemonSetGetTask) Name() string { return "kubernetes.daemonset.get" }

func (t *daemonSetGetTask) JSONSchema() string { return daemonSetGetSchema }

func (t *daemonSetGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	ds, err := client.clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"daemonset": flattenDaemonSetDetail(*ds)}), nil
}

func flattenDaemonSetDetail(ds appsv1.DaemonSet) daemonSetDetail {
	return daemonSetDetail{
		Name:      ds.Name,
		Namespace: ds.Namespace,
		Desired:   int(ds.Status.DesiredNumberScheduled),
		Current:   int(ds.Status.CurrentNumberScheduled),
		Ready:     int(ds.Status.NumberReady),
		Updated:   int(ds.Status.UpdatedNumberScheduled),
		Available: int(ds.Status.NumberAvailable),
		Age:       formatAge(ds.CreationTimestamp.Time),
		Labels:    ds.Labels,
	}
}

const daemonSetGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "DaemonSet Get Parameters",
  "description": "Parameters for retrieving a specific daemonset.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the daemonset."
    },
    "name": {
      "type": "string",
      "description": "Name of the daemonset."
    }
  },
  "required": ["namespace", "name"]
}`

type daemonSetDetail struct {
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Desired   int               `json:"desired,omitempty"`
	Current   int               `json:"current,omitempty"`
	Ready     int               `json:"ready,omitempty"`
	Updated   int               `json:"updated,omitempty"`
	Available int               `json:"available,omitempty"`
	Age       string            `json:"age,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}
