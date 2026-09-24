package kubernetes

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*jobListTask)(nil)

type jobListTask struct{ provider *Provider }

func (t *jobListTask) Name() string { return "kubernetes.job.list" }

func (t *jobListTask) JSONSchema() string { return jobListSchema }

func (t *jobListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.BatchV1().Jobs(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []jobSummary
	for _, job := range list.Items {
		items = append(items, flattenJob(job))
	}

	return common.SuccessResult(map[string]any{"jobs": items, "count": len(items)}), nil
}

func flattenJob(job batchv1.Job) jobSummary {
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
	return jobSummary{
		Name:        job.Name,
		Namespace:   job.Namespace,
		Completions: fmt.Sprintf("%d/%d", succeeded, completions),
		Duration:    duration,
		Age:         formatAge(job.CreationTimestamp.Time),
		Conditions:  conditions,
	}
}

const jobListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Job List Parameters",
  "description": "Parameters for listing jobs.",
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

type jobSummary struct {
	Name        string             `json:"name,omitempty"`
	Namespace   string             `json:"namespace,omitempty"`
	Completions string             `json:"completions,omitempty"`
	Duration    string             `json:"duration,omitempty"`
	Age         string             `json:"age,omitempty"`
	Conditions  []conditionSummary `json:"conditions,omitempty"`
}
