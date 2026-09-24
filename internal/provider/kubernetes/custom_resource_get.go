package kubernetes

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*customResourceGetTask)(nil)

type customResourceGetTask struct{ provider *Provider }

func (t *customResourceGetTask) Name() string { return "kubernetes.custom_resource.get" }

func (t *customResourceGetTask) JSONSchema() string { return customResourceGetSchema }

func (t *customResourceGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	apiGroup, err := common.RequireString(params, "api_group")
	if err != nil {
		return common.TaskFailure(err)
	}
	version, err := common.RequireString(params, "version")
	if err != nil {
		return common.TaskFailure(err)
	}
	resource, err := common.RequireString(params, "resource")
	if err != nil {
		return common.TaskFailure(err)
	}
	namespace, err := common.RequireString(params, "namespace")
	if err != nil {
		return common.TaskFailure(err)
	}
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	client := t.provider.CurrentClient()
	if client == nil || client.dynamicClient == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	item, err := client.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  version,
		Resource: resource,
	}).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"object": item.Object}), nil
}

const customResourceGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Custom Resource Get Parameters",
  "description": "Parameters for retrieving a specific Kubernetes custom resource using the dynamic API client.",
  "properties": {
    "api_group": {
      "type": "string",
      "description": "API group of the custom resource, such as cert-manager.io."
    },
    "version": {
      "type": "string",
      "description": "API version of the custom resource, such as v1."
    },
    "resource": {
      "type": "string",
      "description": "Plural resource name, such as certificates."
    },
    "namespace": {
      "type": "string",
      "description": "Namespace containing the custom resource."
    },
    "name": {
      "type": "string",
      "description": "Name of the custom resource object."
    }
  },
  "required": ["api_group", "version", "resource", "namespace", "name"]
}`
