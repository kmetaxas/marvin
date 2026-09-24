package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*serviceAccountListTask)(nil)

type serviceAccountListTask struct{ provider *Provider }

func (t *serviceAccountListTask) Name() string { return "kubernetes.serviceaccount.list" }

func (t *serviceAccountListTask) JSONSchema() string { return serviceAccountListSchema }

func (t *serviceAccountListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.OptionalString(params, "namespace", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	labelSelector, err := common.OptionalString(params, "label_selector", "")
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
		Limit:         int64(limit),
	}

	list, err := client.clientset.CoreV1().ServiceAccounts(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []serviceAccountSummary
	for _, sa := range list.Items {
		items = append(items, flattenServiceAccount(sa))
	}

	return common.SuccessResult(map[string]any{"service_accounts": items, "count": len(items)}), nil
}

func flattenServiceAccount(sa corev1.ServiceAccount) serviceAccountSummary {
	secrets := make([]string, 0, len(sa.Secrets))
	for _, s := range sa.Secrets {
		secrets = append(secrets, s.Name)
	}
	imagePullSecrets := make([]string, 0, len(sa.ImagePullSecrets))
	for _, s := range sa.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, s.Name)
	}
	return serviceAccountSummary{
		Name:             sa.Name,
		Namespace:        sa.Namespace,
		Secrets:          secrets,
		ImagePullSecrets: imagePullSecrets,
		Age:              formatAge(sa.CreationTimestamp.Time),
	}
}

const serviceAccountListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ServiceAccount List Parameters",
  "description": "Parameters for listing ServiceAccounts.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit to list across all namespaces."
    },
    "label_selector": {
      "type": "string",
      "description": "Label selector expression."
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

type serviceAccountSummary struct {
	Name             string   `json:"name,omitempty"`
	Namespace        string   `json:"namespace,omitempty"`
	Secrets          []string `json:"secrets,omitempty"`
	ImagePullSecrets []string `json:"image_pull_secrets,omitempty"`
	Age              string   `json:"age,omitempty"`
}
