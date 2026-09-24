package kubernetes

import (
	"context"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ task.Task = (*ingressListTask)(nil)

type ingressListTask struct{ provider *Provider }

func (t *ingressListTask) Name() string { return "kubernetes.ingress.list" }

func (t *ingressListTask) JSONSchema() string { return ingressListSchema }

func (t *ingressListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	list, err := client.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
		Limit:         int64(limit),
	})
	if err != nil {
		return common.TaskFailure(err)
	}

	items := make([]ingressSummary, 0, len(list.Items))
	for _, ingress := range list.Items {
		items = append(items, flattenIngressSummary(ingress))
	}

	return common.SuccessResult(map[string]any{"ingresses": items, "count": len(items)}), nil
}

func flattenIngressSummary(ingress networkingv1.Ingress) ingressSummary {
	hosts := make([]string, 0, len(ingress.Spec.Rules))
	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}

	tlsHosts := make([]string, 0)
	for _, tls := range ingress.Spec.TLS {
		tlsHosts = append(tlsHosts, tls.Hosts...)
	}

	class := ""
	if ingress.Spec.IngressClassName != nil {
		class = *ingress.Spec.IngressClassName
	}

	return ingressSummary{
		Name:       ingress.Name,
		Namespace:  ingress.Namespace,
		Class:      class,
		Hosts:      hosts,
		TLSHosts:   tlsHosts,
		RulesCount: len(ingress.Spec.Rules),
		Age:        formatAge(ingress.CreationTimestamp.Time),
	}
}

const ingressListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Ingress List Parameters",
  "description": "Parameters for listing Kubernetes ingresses.",
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

type ingressSummary struct {
	Name       string   `json:"name,omitempty"`
	Namespace  string   `json:"namespace,omitempty"`
	Class      string   `json:"class,omitempty"`
	Hosts      []string `json:"hosts,omitempty"`
	TLSHosts   []string `json:"tls_hosts,omitempty"`
	RulesCount int      `json:"rules_count,omitempty"`
	Age        string   `json:"age,omitempty"`
}
