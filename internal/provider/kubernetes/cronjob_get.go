package kubernetes

import (
	"context"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*cronJobGetTask)(nil)

type cronJobGetTask struct{ provider *Provider }

func (t *cronJobGetTask) Name() string { return "kubernetes.cronjob.get" }

func (t *cronJobGetTask) JSONSchema() string { return cronJobGetSchema }

func (t *cronJobGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	cj, err := client.clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"cronjob": flattenCronJobDetail(*cj)}), nil
}

func flattenCronJobDetail(cj batchv1.CronJob) cronJobDetail {
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
	return cronJobDetail{
		Name:               cj.Name,
		Namespace:          cj.Namespace,
		Schedule:           cj.Spec.Schedule,
		Suspend:            suspend,
		Active:             len(cj.Status.Active),
		LastScheduleTime:   lastSchedule,
		LastSuccessfulTime: lastSuccessful,
		Age:                formatAge(cj.CreationTimestamp.Time),
		Labels:             cj.Labels,
	}
}

const cronJobGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "CronJob Get Parameters",
  "description": "Parameters for retrieving a specific cronjob.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the cronjob."
    },
    "name": {
      "type": "string",
      "description": "Name of the cronjob."
    }
  },
  "required": ["namespace", "name"]
}`

type cronJobDetail struct {
	Name               string            `json:"name,omitempty"`
	Namespace          string            `json:"namespace,omitempty"`
	Schedule           string            `json:"schedule,omitempty"`
	Suspend            bool              `json:"suspend,omitempty"`
	Active             int               `json:"active,omitempty"`
	LastScheduleTime   string            `json:"last_schedule_time,omitempty"`
	LastSuccessfulTime string            `json:"last_successful_time,omitempty"`
	Age                string            `json:"age,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}
