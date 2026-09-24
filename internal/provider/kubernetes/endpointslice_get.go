package kubernetes

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*endpointSliceGetTask)(nil)

type endpointSliceGetTask struct{ provider *Provider }

func (t *endpointSliceGetTask) Name() string { return "kubernetes.endpointslice.get" }

func (t *endpointSliceGetTask) JSONSchema() string { return endpointSliceGetSchema }

func (t *endpointSliceGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	endpointSlice, err := client.clientset.DiscoveryV1().EndpointSlices(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"endpoint_slice": flattenEndpointSliceDetail(*endpointSlice)}), nil
}

func flattenEndpointSliceDetail(endpointSlice discoveryv1.EndpointSlice) endpointSliceDetail {
	return endpointSliceDetail(flattenEndpointSlice(endpointSlice))
}

const endpointSliceGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "EndpointSlice Get Parameters",
  "description": "Parameters for retrieving a specific EndpointSlice.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the EndpointSlice."
    },
    "name": {
      "type": "string",
      "description": "Name of the EndpointSlice."
    }
  },
  "required": ["namespace", "name"]
}`

type endpointSliceDetail struct {
	Name        string                     `json:"name,omitempty"`
	Namespace   string                     `json:"namespace,omitempty"`
	ServiceName string                     `json:"service_name,omitempty"`
	Ports       []endpointSlicePortSummary `json:"ports,omitempty"`
	Endpoints   []endpointSummary          `json:"endpoints,omitempty"`
	Age         string                     `json:"age,omitempty"`
}
