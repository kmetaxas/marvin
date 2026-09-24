package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*serviceAccountGetTask)(nil)

type serviceAccountGetTask struct{ provider *Provider }

func (t *serviceAccountGetTask) Name() string { return "kubernetes.serviceaccount.get" }

func (t *serviceAccountGetTask) JSONSchema() string { return serviceAccountGetSchema }

func (t *serviceAccountGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	sa, err := client.clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"service_account": flattenServiceAccount(*sa)}), nil
}

const serviceAccountGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "ServiceAccount Get Parameters",
  "description": "Parameters for retrieving a specific ServiceAccount.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the ServiceAccount."
    },
    "name": {
      "type": "string",
      "description": "Name of the ServiceAccount."
    }
  },
  "required": ["namespace", "name"]
}`
