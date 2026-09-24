package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*serviceGetTask)(nil)

type serviceGetTask struct{ provider *Provider }

func (t *serviceGetTask) Name() string { return "kubernetes.service.get" }

func (t *serviceGetTask) JSONSchema() string { return serviceGetSchema }

func (t *serviceGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	svc, err := client.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"service": flattenServiceDetail(*svc)}), nil
}

func flattenServiceDetail(svc corev1.Service) serviceDetail {
	loadBalancerIPs := make([]string, 0, len(svc.Status.LoadBalancer.Ingress))
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			loadBalancerIPs = append(loadBalancerIPs, ingress.IP)
		}
	}

	return serviceDetail{
		Name:                  svc.Name,
		Namespace:             svc.Namespace,
		Type:                  string(svc.Spec.Type),
		ClusterIP:             svc.Spec.ClusterIP,
		ExternalIPs:           svc.Spec.ExternalIPs,
		Ports:                 flattenServicePorts(svc.Spec.Ports),
		Selector:              svc.Spec.Selector,
		Age:                   formatAge(svc.CreationTimestamp.Time),
		LoadBalancerIPs:       loadBalancerIPs,
		SessionAffinity:       string(svc.Spec.SessionAffinity),
		ExternalTrafficPolicy: string(svc.Spec.ExternalTrafficPolicy),
	}
}

const serviceGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Service Get Parameters",
  "description": "Parameters for retrieving a specific service.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the service."
    },
    "name": {
      "type": "string",
      "description": "Name of the service."
    }
  },
  "required": ["namespace", "name"]
}`

type serviceDetail struct {
	Name                  string               `json:"name,omitempty"`
	Namespace             string               `json:"namespace,omitempty"`
	Type                  string               `json:"type,omitempty"`
	ClusterIP             string               `json:"cluster_ip,omitempty"`
	ExternalIPs           []string             `json:"external_ips,omitempty"`
	Ports                 []servicePortSummary `json:"ports,omitempty"`
	Selector              map[string]string    `json:"selector,omitempty"`
	Age                   string               `json:"age,omitempty"`
	LoadBalancerIPs       []string             `json:"load_balancer_ips,omitempty"`
	SessionAffinity       string               `json:"session_affinity,omitempty"`
	ExternalTrafficPolicy string               `json:"external_traffic_policy,omitempty"`
}
