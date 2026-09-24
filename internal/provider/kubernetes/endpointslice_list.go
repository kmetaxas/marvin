package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*endpointSliceListTask)(nil)

type endpointSliceListTask struct{ provider *Provider }

func (t *endpointSliceListTask) Name() string { return "kubernetes.endpointslice.list" }

func (t *endpointSliceListTask) JSONSchema() string { return endpointSliceListSchema }

func (t *endpointSliceListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []endpointSliceSummary
	for _, endpointSlice := range list.Items {
		items = append(items, flattenEndpointSlice(endpointSlice))
	}

	return common.SuccessResult(map[string]any{"endpoint_slices": items, "count": len(items)}), nil
}

func flattenEndpointSlice(endpointSlice discoveryv1.EndpointSlice) endpointSliceSummary {
	return endpointSliceSummary{
		Name:        endpointSlice.Name,
		Namespace:   endpointSlice.Namespace,
		ServiceName: endpointSlice.Labels[discoveryv1.LabelServiceName],
		Ports:       flattenEndpointSlicePorts(endpointSlice.Ports),
		Endpoints:   flattenEndpointSliceEndpoints(endpointSlice.Endpoints),
		Age:         formatAge(endpointSlice.CreationTimestamp.Time),
	}
}

func flattenEndpointSlicePorts(ports []discoveryv1.EndpointPort) []endpointSlicePortSummary {
	items := make([]endpointSlicePortSummary, 0, len(ports))
	for _, port := range ports {
		items = append(items, endpointSlicePortSummary{
			Name:     valueOrEmpty(port.Name),
			Port:     int32Value(port.Port),
			Protocol: protocolValue(port.Protocol),
		})
	}
	return items
}

func flattenEndpointSliceEndpoints(endpoints []discoveryv1.Endpoint) []endpointSummary {
	items := make([]endpointSummary, 0, len(endpoints))
	for _, endpoint := range endpoints {
		items = append(items, endpointSummary{
			Address:     firstAddress(endpoint.Addresses),
			Ready:       boolValue(endpoint.Conditions.Ready),
			Serving:     boolValue(endpoint.Conditions.Serving),
			Terminating: boolValue(endpoint.Conditions.Terminating),
			NodeName:    valueOrEmpty(endpoint.NodeName),
		})
	}
	return items
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func boolValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func protocolValue(value *corev1.Protocol) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func firstAddress(addresses []string) string {
	if len(addresses) == 0 {
		return ""
	}
	return addresses[0]
}

const endpointSliceListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "EndpointSlice List Parameters",
  "description": "Parameters for listing EndpointSlices.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace to filter. Omit to list across all namespaces."
    },
    "label_selector": {
      "type": "string",
      "description": "Label selector expression (e.g., 'app=nginx')."
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

type endpointSliceSummary struct {
	Name        string                     `json:"name,omitempty"`
	Namespace   string                     `json:"namespace,omitempty"`
	ServiceName string                     `json:"service_name,omitempty"`
	Ports       []endpointSlicePortSummary `json:"ports,omitempty"`
	Endpoints   []endpointSummary          `json:"endpoints,omitempty"`
	Age         string                     `json:"age,omitempty"`
}

type endpointSlicePortSummary struct {
	Name     string `json:"name,omitempty"`
	Port     int32  `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

type endpointSummary struct {
	Address     string `json:"address,omitempty"`
	Ready       bool   `json:"ready,omitempty"`
	Serving     bool   `json:"serving,omitempty"`
	Terminating bool   `json:"terminating,omitempty"`
	NodeName    string `json:"node_name,omitempty"`
}
