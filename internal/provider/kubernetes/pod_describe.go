package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*podDescribeTask)(nil)

type podDescribeTask struct{ provider *Provider }

func (t *podDescribeTask) Name() string { return "kubernetes.pod.describe" }

func (t *podDescribeTask) JSONSchema() string { return podDescribeSchema }

func (t *podDescribeTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.RequireString(params, "namespace")
	if err != nil {
		return common.TaskFailure(err)
	}
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	eventLimit, err := common.OptionalInt(params, "event_limit", 20)
	if err != nil {
		return common.TaskFailure(err)
	}
	if eventLimit <= 0 {
		return common.TaskFailure(fmt.Errorf("parameter event_limit must be greater than 0"))
	}
	if eventLimit > 100 {
		eventLimit = 100
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	pod, err := client.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}
	events, err := client.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", name),
		Limit:         int64(eventLimit),
	})
	if err != nil {
		return common.TaskFailure(err)
	}

	items := make([]eventSummary, 0, len(events.Items))
	for _, event := range events.Items {
		if event.InvolvedObject.Name != name || event.InvolvedObject.Kind != "Pod" {
			continue
		}
		items = append(items, flattenEvent(event))
		if len(items) >= eventLimit {
			break
		}
	}

	return common.SuccessResult(map[string]any{
		"pod":         flattenPodDetail(*pod),
		"events":      items,
		"event_count": len(items),
	}), nil
}

const podDescribeSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Pod Describe Parameters",
  "description": "Parameters for describing a pod with detailed status and recent events.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the pod."
    },
    "name": {
      "type": "string",
      "description": "Name of the pod."
    },
    "event_limit": {
      "type": "integer",
      "description": "Maximum number of recent events to return for this pod.",
      "minimum": 1,
      "maximum": 100,
      "default": 20
    }
  },
  "required": ["namespace", "name"]
}`
