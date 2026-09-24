package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*eventListTask)(nil)

type eventListTask struct{ provider *Provider }

func (t *eventListTask) Name() string { return "kubernetes.event.list" }

func (t *eventListTask) JSONSchema() string { return eventListSchema }

func (t *eventListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.OptionalString(params, "namespace", "")
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
		FieldSelector: fieldSelector,
		Limit:         int64(limit),
	}

	list, err := client.clientset.CoreV1().Events(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []eventSummary
	for _, ev := range list.Items {
		items = append(items, flattenEvent(ev))
	}

	return common.SuccessResult(map[string]any{"events": items, "count": len(items)}), nil
}

func flattenEvent(ev corev1.Event) eventSummary {
	involved := ""
	if ev.InvolvedObject.Kind != "" && ev.InvolvedObject.Name != "" {
		involved = fmt.Sprintf("%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name)
	}
	return eventSummary{
		Name:           ev.Name,
		Namespace:      ev.Namespace,
		Type:           ev.Type,
		Reason:         ev.Reason,
		Message:        ev.Message,
		Source:         ev.Source.Component,
		FirstTimestamp: ev.FirstTimestamp.Format(time.RFC3339),
		LastTimestamp:  ev.LastTimestamp.Format(time.RFC3339),
		Count:          int(ev.Count),
		InvolvedObject: involved,
	}
}

const eventListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Event List Parameters",
  "description": "Parameters for listing Kubernetes events.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit to list across all namespaces."
    },
    "field_selector": {
      "type": "string",
      "description": "Field selector expression. Useful for filtering by involved object (e.g., 'involvedObject.name=pod-xyz')."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum number of events to return.",
      "minimum": 1,
      "maximum": 1000,
      "default": 100
    }
  }
}`

type eventSummary struct {
	Name           string `json:"name,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	Type           string `json:"type,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Message        string `json:"message,omitempty"`
	Source         string `json:"source,omitempty"`
	FirstTimestamp string `json:"first_timestamp,omitempty"`
	LastTimestamp  string `json:"last_timestamp,omitempty"`
	Count          int    `json:"count,omitempty"`
	InvolvedObject string `json:"involved_object,omitempty"`
}
