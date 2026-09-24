package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*endpointsListTask)(nil)

type endpointsListTask struct{ provider *Provider }

func (t *endpointsListTask) Name() string { return "kubernetes.endpoints.list" }

func (t *endpointsListTask) JSONSchema() string { return endpointsListSchema }

func (t *endpointsListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.CoreV1().Endpoints(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []endpointsSummary
	for _, endpoints := range list.Items {
		items = append(items, flattenEndpoints(endpoints))
	}

	return common.SuccessResult(map[string]any{"endpoints": items, "count": len(items)}), nil
}

func flattenEndpoints(endpoints corev1.Endpoints) endpointsSummary {
	return endpointsSummary{
		Name:      endpoints.Name,
		Namespace: endpoints.Namespace,
		Subsets:   flattenEndpointsSubsets(endpoints.Subsets),
		Age:       formatAge(endpoints.CreationTimestamp.Time),
	}
}

func flattenEndpointsSubsets(subsets []corev1.EndpointSubset) []endpointsSubsetSummary {
	items := make([]endpointsSubsetSummary, 0, len(subsets))
	for _, subset := range subsets {
		items = append(items, endpointsSubsetSummary{
			Addresses:         flattenEndpointAddresses(subset.Addresses),
			Ports:             flattenEndpointsPorts(subset.Ports),
			NotReadyAddresses: flattenEndpointAddresses(subset.NotReadyAddresses),
		})
	}
	return items
}

func flattenEndpointAddresses(addresses []corev1.EndpointAddress) []string {
	items := make([]string, 0, len(addresses))
	for _, address := range addresses {
		items = append(items, address.IP)
	}
	return items
}

func flattenEndpointsPorts(ports []corev1.EndpointPort) []endpointsPortSummary {
	items := make([]endpointsPortSummary, 0, len(ports))
	for _, port := range ports {
		items = append(items, endpointsPortSummary{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: string(port.Protocol),
		})
	}
	return items
}

const endpointsListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Endpoints List Parameters",
  "description": "Parameters for listing endpoints.",
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
      "description": "Field selector expression."
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

type endpointsSummary struct {
	Name      string                   `json:"name,omitempty"`
	Namespace string                   `json:"namespace,omitempty"`
	Subsets   []endpointsSubsetSummary `json:"subsets,omitempty"`
	Age       string                   `json:"age,omitempty"`
}

type endpointsSubsetSummary struct {
	Addresses         []string               `json:"addresses,omitempty"`
	Ports             []endpointsPortSummary `json:"ports,omitempty"`
	NotReadyAddresses []string               `json:"not_ready_addresses,omitempty"`
}

type endpointsPortSummary struct {
	Name     string `json:"name,omitempty"`
	Port     int32  `json:"port,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}
