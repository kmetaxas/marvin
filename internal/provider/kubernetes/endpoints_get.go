package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*endpointsGetTask)(nil)

type endpointsGetTask struct{ provider *Provider }

func (t *endpointsGetTask) Name() string { return "kubernetes.endpoints.get" }

func (t *endpointsGetTask) JSONSchema() string { return endpointsGetSchema }

func (t *endpointsGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	endpoints, err := client.clientset.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"endpoints": flattenEndpointsDetail(*endpoints)}), nil
}

func flattenEndpointsDetail(endpoints corev1.Endpoints) endpointsDetail {
	return endpointsDetail(flattenEndpoints(endpoints))
}

const endpointsGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Endpoints Get Parameters",
  "description": "Parameters for retrieving specific endpoints.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the endpoints."
    },
    "name": {
      "type": "string",
      "description": "Name of the endpoints."
    }
  },
  "required": ["namespace", "name"]
}`

type endpointsDetail endpointsSummary
