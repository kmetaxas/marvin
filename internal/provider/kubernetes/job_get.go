package kubernetes

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*jobGetTask)(nil)

type jobGetTask struct{ provider *Provider }

func (t *jobGetTask) Name() string { return "kubernetes.job.get" }

func (t *jobGetTask) JSONSchema() string { return jobGetSchema }

func (t *jobGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	job, err := client.clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"job": flattenJobDetail(*job)}), nil
}

func flattenJobDetail(job batchv1.Job) jobDetail {
	succeeded := int(job.Status.Succeeded)
	completions := succeeded
	if job.Spec.Completions != nil {
		completions = int(*job.Spec.Completions)
	}
	var conditions []conditionSummary
	for _, c := range job.Status.Conditions {
		conditions = append(conditions, conditionSummary{
			Type:   string(c.Type),
			Status: string(c.Status),
		})
	}
	duration := ""
	if job.Status.CompletionTime != nil && job.Status.StartTime != nil {
		duration = job.Status.CompletionTime.Sub(job.Status.StartTime.Time).String()
	}
	return jobDetail{
		Name:        job.Name,
		Namespace:   job.Namespace,
		Completions: fmt.Sprintf("%d/%d", succeeded, completions),
		Duration:    duration,
		Age:         formatAge(job.CreationTimestamp.Time),
		Conditions:  conditions,
		Labels:      job.Labels,
	}
}

const jobGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Job Get Parameters",
  "description": "Parameters for retrieving a specific job.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the job."
    },
    "name": {
      "type": "string",
      "description": "Name of the job."
    }
  },
  "required": ["namespace", "name"]
}`

type jobDetail struct {
	Name        string             `json:"name,omitempty"`
	Namespace   string             `json:"namespace,omitempty"`
	Completions string             `json:"completions,omitempty"`
	Duration    string             `json:"duration,omitempty"`
	Age         string             `json:"age,omitempty"`
	Conditions  []conditionSummary `json:"conditions,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty"`
}
