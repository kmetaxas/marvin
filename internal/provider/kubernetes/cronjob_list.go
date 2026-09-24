package kubernetes

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*cronJobListTask)(nil)

type cronJobListTask struct{ provider *Provider }

func (t *cronJobListTask) Name() string { return "kubernetes.cronjob.list" }

func (t *cronJobListTask) JSONSchema() string { return cronJobListSchema }

func (t *cronJobListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.BatchV1().CronJobs(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []cronJobSummary
	for _, cj := range list.Items {
		items = append(items, flattenCronJob(cj))
	}

	return common.SuccessResult(map[string]any{"cronjobs": items, "count": len(items)}), nil
}

func flattenCronJob(cj batchv1.CronJob) cronJobSummary {
	suspend := false
	if cj.Spec.Suspend != nil {
		suspend = *cj.Spec.Suspend
	}
	lastSchedule := ""
	if cj.Status.LastScheduleTime != nil {
		lastSchedule = cj.Status.LastScheduleTime.Format(time.RFC3339)
	}
	lastSuccessful := ""
	if cj.Status.LastSuccessfulTime != nil {
		lastSuccessful = cj.Status.LastSuccessfulTime.Format(time.RFC3339)
	}
	return cronJobSummary{
		Name:               cj.Name,
		Namespace:          cj.Namespace,
		Schedule:           cj.Spec.Schedule,
		Suspend:            suspend,
		Active:             len(cj.Status.Active),
		LastScheduleTime:   lastSchedule,
		LastSuccessfulTime: lastSuccessful,
		Age:                formatAge(cj.CreationTimestamp.Time),
	}
}

const cronJobListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "CronJob List Parameters",
  "description": "Parameters for listing cronjobs.",
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

type cronJobSummary struct {
	Name               string `json:"name,omitempty"`
	Namespace          string `json:"namespace,omitempty"`
	Schedule           string `json:"schedule,omitempty"`
	Suspend            bool   `json:"suspend,omitempty"`
	Active             int    `json:"active,omitempty"`
	LastScheduleTime   string `json:"last_schedule_time,omitempty"`
	LastSuccessfulTime string `json:"last_successful_time,omitempty"`
	Age                string `json:"age,omitempty"`
}
