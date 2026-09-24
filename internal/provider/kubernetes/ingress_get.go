package kubernetes

import (
	"context"
	"fmt"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*ingressGetTask)(nil)

type ingressGetTask struct{ provider *Provider }

func (t *ingressGetTask) Name() string { return "kubernetes.ingress.get" }

func (t *ingressGetTask) JSONSchema() string { return ingressGetSchema }

func (t *ingressGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	ingress, err := client.clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"ingress": flattenIngressDetail(*ingress)}), nil
}

func flattenIngressDetail(ingress networkingv1.Ingress) ingressDetail {
	rules := make([]ingressRuleDetail, 0, len(ingress.Spec.Rules))
	for _, rule := range ingress.Spec.Rules {
		paths := make([]ingressPathDetail, 0)
		if rule.HTTP != nil {
			paths = make([]ingressPathDetail, 0, len(rule.HTTP.Paths))
			for _, path := range rule.HTTP.Paths {
				pathType := ""
				if path.PathType != nil {
					pathType = string(*path.PathType)
				}
				paths = append(paths, ingressPathDetail{
					Path:     path.Path,
					PathType: pathType,
					Backend:  ingressBackendString(path.Backend),
				})
			}
		}
		rules = append(rules, ingressRuleDetail{Host: rule.Host, Paths: paths})
	}

	tls := make([]ingressTLSDetail, 0, len(ingress.Spec.TLS))
	for _, entry := range ingress.Spec.TLS {
		tls = append(tls, ingressTLSDetail{Hosts: entry.Hosts, SecretName: entry.SecretName})
	}

	class := ""
	if ingress.Spec.IngressClassName != nil {
		class = *ingress.Spec.IngressClassName
	}

	return ingressDetail{
		Name:      ingress.Name,
		Namespace: ingress.Namespace,
		Class:     class,
		Rules:     rules,
		TLS:       tls,
		Status: ingressStatusDetail{
			LoadBalancer: ingressLoadBalancerString(ingress.Status.LoadBalancer.Ingress),
		},
		Age: formatAge(ingress.CreationTimestamp.Time),
	}
}

func ingressBackendString(backend networkingv1.IngressBackend) string {
	if backend.Service == nil {
		return ""
	}
	port := backend.Service.Port.Name
	if port == "" {
		port = fmt.Sprintf("%d", backend.Service.Port.Number)
	}
	return fmt.Sprintf("Service/%s:%s", backend.Service.Name, port)
}

func ingressLoadBalancerString(items []networkingv1.IngressLoadBalancerIngress) string {
	if len(items) == 0 {
		return ""
	}
	if items[0].IP != "" {
		return items[0].IP
	}
	return items[0].Hostname
}

const ingressGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Ingress Get Parameters",
  "description": "Parameters for retrieving a specific Kubernetes ingress.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the ingress."
    },
    "name": {
      "type": "string",
      "description": "Name of the ingress."
    }
  },
  "required": ["namespace", "name"]
}`

type ingressDetail struct {
	Name      string              `json:"name,omitempty"`
	Namespace string              `json:"namespace,omitempty"`
	Class     string              `json:"class,omitempty"`
	Rules     []ingressRuleDetail `json:"rules,omitempty"`
	TLS       []ingressTLSDetail  `json:"tls,omitempty"`
	Status    ingressStatusDetail `json:"status"`
	Age       string              `json:"age,omitempty"`
}

type ingressRuleDetail struct {
	Host  string              `json:"host,omitempty"`
	Paths []ingressPathDetail `json:"paths,omitempty"`
}

type ingressPathDetail struct {
	Path     string `json:"path,omitempty"`
	PathType string `json:"path_type,omitempty"`
	Backend  string `json:"backend,omitempty"`
}

type ingressTLSDetail struct {
	Hosts      []string `json:"hosts,omitempty"`
	SecretName string   `json:"secret_name,omitempty"`
}

type ingressStatusDetail struct {
	LoadBalancer string `json:"load_balancer,omitempty"`
}
