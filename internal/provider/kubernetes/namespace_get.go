package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*namespaceGetTask)(nil)

type namespaceGetTask struct{ provider *Provider }

func (t *namespaceGetTask) Name() string { return "kubernetes.namespace.get" }

func (t *namespaceGetTask) JSONSchema() string { return namespaceGetSchema }

func (t *namespaceGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	ns, err := client.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"namespace": flattenNamespace(*ns)}), nil
}

const namespaceGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Namespace Get Parameters",
  "description": "Parameters for retrieving a specific namespace.",
  "properties": {
    "name": {
      "type": "string",
      "description": "Name of the namespace."
    }
  },
  "required": ["name"]
}`
