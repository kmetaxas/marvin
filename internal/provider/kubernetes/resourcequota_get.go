package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*resourceQuotaGetTask)(nil)

type resourceQuotaGetTask struct{ provider *Provider }

func (t *resourceQuotaGetTask) Name() string { return "kubernetes.resourcequota.get" }

func (t *resourceQuotaGetTask) JSONSchema() string { return resourceQuotaGetSchema }

func (t *resourceQuotaGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	rq, err := client.clientset.CoreV1().ResourceQuotas(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"resource_quota": flattenResourceQuotaDetail(*rq)}), nil
}

func flattenResourceQuotaDetail(rq corev1.ResourceQuota) resourceQuotaDetail {
	return resourceQuotaDetail{
		Name:      rq.Name,
		Namespace: rq.Namespace,
		Hard:      flattenResourceList(rq.Status.Hard),
		Used:      flattenResourceList(rq.Status.Used),
		Age:       formatAge(rq.CreationTimestamp.Time),
	}
}

const resourceQuotaGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ResourceQuota Get Parameters",
  "description": "Parameters for retrieving a specific ResourceQuota.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the ResourceQuota."
    },
    "name": {
      "type": "string",
      "description": "Name of the ResourceQuota."
    }
  },
  "required": ["namespace", "name"]
}`

type resourceQuotaDetail struct {
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Hard      map[string]string `json:"hard,omitempty"`
	Used      map[string]string `json:"used,omitempty"`
	Age       string            `json:"age,omitempty"`
}
