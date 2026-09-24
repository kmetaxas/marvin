package kubernetes

import (
	"context"
	"fmt"
	"strconv"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ task.Task = (*serviceListTask)(nil)

type serviceListTask struct{ provider *Provider }

func (t *serviceListTask) Name() string { return "kubernetes.service.list" }

func (t *serviceListTask) JSONSchema() string { return serviceListSchema }

func (t *serviceListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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
	serviceType, err := common.OptionalString(params, "type", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	if serviceType != "" && !isValidServiceType(serviceType) {
		return common.TaskFailure(fmt.Errorf("parameter type must be one of ClusterIP, NodePort, LoadBalancer, ExternalName"))
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

	list, err := client.clientset.CoreV1().Services(namespace).List(ctx, opts)
	if err != nil {
		return common.TaskFailure(err)
	}

	var items []serviceSummary
	for _, svc := range list.Items {
		if serviceType != "" && svc.Spec.Type != corev1.ServiceType(serviceType) {
			continue
		}
		items = append(items, flattenServiceSummary(svc))
	}

	return common.SuccessResult(map[string]any{"services": items, "count": len(items)}), nil
}

func isValidServiceType(serviceType string) bool {
	switch corev1.ServiceType(serviceType) {
	case corev1.ServiceTypeClusterIP, corev1.ServiceTypeNodePort, corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeExternalName:
		return true
	default:
		return false
	}
}

func flattenServiceSummary(svc corev1.Service) serviceSummary {
	return serviceSummary{
		Name:        svc.Name,
		Namespace:   svc.Namespace,
		Type:        string(svc.Spec.Type),
		ClusterIP:   svc.Spec.ClusterIP,
		ExternalIPs: svc.Spec.ExternalIPs,
		Ports:       flattenServicePorts(svc.Spec.Ports),
		Selector:    svc.Spec.Selector,
		Age:         formatAge(svc.CreationTimestamp.Time),
	}
}

func flattenServicePorts(ports []corev1.ServicePort) []servicePortSummary {
	items := make([]servicePortSummary, 0, len(ports))
	for _, port := range ports {
		items = append(items, servicePortSummary{
			Name:       port.Name,
			Port:       port.Port,
			TargetPort: formatTargetPort(port.TargetPort),
			Protocol:   string(port.Protocol),
		})
	}
	return items
}

func formatTargetPort(target intstr.IntOrString) string {
	if target.Type == intstr.String {
		return target.StrVal
	}
	if target.IntValue() != 0 {
		return strconv.Itoa(target.IntValue())
	}
	return ""
}

const serviceListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Service List Parameters",
  "description": "Parameters for listing services.",
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
    "type": {
      "type": "string",
      "description": "Service type to include in results.",
      "enum": ["ClusterIP", "NodePort", "LoadBalancer", "ExternalName"]
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

type serviceSummary struct {
	Name        string               `json:"name,omitempty"`
	Namespace   string               `json:"namespace,omitempty"`
	Type        string               `json:"type,omitempty"`
	ClusterIP   string               `json:"cluster_ip,omitempty"`
	ExternalIPs []string             `json:"external_ips,omitempty"`
	Ports       []servicePortSummary `json:"ports,omitempty"`
	Selector    map[string]string    `json:"selector,omitempty"`
	Age         string               `json:"age,omitempty"`
}

type servicePortSummary struct {
	Name       string `json:"name,omitempty"`
	Port       int32  `json:"port,omitempty"`
	TargetPort string `json:"target_port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}
