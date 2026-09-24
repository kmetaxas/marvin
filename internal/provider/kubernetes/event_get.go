package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*eventGetTask)(nil)

type eventGetTask struct{ provider *Provider }

func (t *eventGetTask) Name() string { return "kubernetes.event.get" }

func (t *eventGetTask) JSONSchema() string { return eventGetSchema }

func (t *eventGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	ev, err := client.clientset.CoreV1().Events(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"event": flattenEvent(*ev)}), nil
}

const eventGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Event Get Parameters",
  "description": "Parameters for retrieving a specific Kubernetes event.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the event."
    },
    "name": {
      "type": "string",
      "description": "Name of the event."
    }
  },
  "required": ["namespace", "name"]
}`
