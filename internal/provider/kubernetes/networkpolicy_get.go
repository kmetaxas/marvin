package kubernetes

import (
	"context"
	"maps"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*networkPolicyGetTask)(nil)

type networkPolicyGetTask struct{ provider *Provider }

func (t *networkPolicyGetTask) Name() string { return "kubernetes.networkpolicy.get" }

func (t *networkPolicyGetTask) JSONSchema() string { return networkPolicyGetSchema }

func (t *networkPolicyGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
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

	networkPolicy, err := client.clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return common.TaskFailure(err)
	}

	return common.SuccessResult(map[string]any{"network_policy": flattenNetworkPolicyDetail(*networkPolicy)}), nil
}

func flattenNetworkPolicyDetail(networkPolicy networkingv1.NetworkPolicy) networkPolicyDetail {
	return networkPolicyDetail{
		Name:        networkPolicy.Name,
		Namespace:   networkPolicy.Namespace,
		PodSelector: matchLabelsOrEmpty(networkPolicy.Spec.PodSelector),
		PolicyTypes: flattenPolicyTypes(networkPolicy.Spec.PolicyTypes),
		Ingress:     flattenIngressRules(networkPolicy.Spec.Ingress),
		Egress:      flattenEgressRules(networkPolicy.Spec.Egress),
		Age:         formatAge(networkPolicy.CreationTimestamp.Time),
	}
}

func flattenIngressRules(rules []networkingv1.NetworkPolicyIngressRule) []networkPolicyRuleDetail {
	items := make([]networkPolicyRuleDetail, 0, len(rules))
	for _, rule := range rules {
		items = append(items, networkPolicyRuleDetail{
			From:  flattenPeers(rule.From),
			Ports: flattenNetworkPolicyPorts(rule.Ports),
		})
	}
	return items
}

func flattenEgressRules(rules []networkingv1.NetworkPolicyEgressRule) []networkPolicyRuleDetail {
	items := make([]networkPolicyRuleDetail, 0, len(rules))
	for _, rule := range rules {
		items = append(items, networkPolicyRuleDetail{
			To:    flattenPeers(rule.To),
			Ports: flattenNetworkPolicyPorts(rule.Ports),
		})
	}
	return items
}

func flattenPeers(peers []networkingv1.NetworkPolicyPeer) []networkPolicyPeerDetail {
	items := make([]networkPolicyPeerDetail, 0, len(peers))
	for _, peer := range peers {
		item := networkPolicyPeerDetail{
			PodSelector:       selectorPointerMatchLabels(peer.PodSelector),
			NamespaceSelector: selectorPointerMatchLabels(peer.NamespaceSelector),
		}
		if peer.IPBlock != nil {
			item.CIDRBlocks = append(item.CIDRBlocks, peer.IPBlock.CIDR)
			item.CIDRBlocks = append(item.CIDRBlocks, peer.IPBlock.Except...)
		}
		items = append(items, item)
	}
	return items
}

func selectorPointerMatchLabels(selector *metav1.LabelSelector) map[string]string {
	if selector == nil || len(selector.MatchLabels) == 0 {
		return map[string]string{}
	}
	labels := make(map[string]string, len(selector.MatchLabels))
	maps.Copy(labels, selector.MatchLabels)
	return labels
}

func flattenNetworkPolicyPorts(ports []networkingv1.NetworkPolicyPort) []networkPolicyPortDetail {
	items := make([]networkPolicyPortDetail, 0, len(ports))
	for _, port := range ports {
		items = append(items, networkPolicyPortDetail{
			Protocol: protocolValue(port.Protocol),
			Port:     networkPolicyPortValue(port.Port),
		})
	}
	return items
}

func networkPolicyPortValue(value *intstr.IntOrString) string {
	if value == nil {
		return ""
	}
	return value.String()
}

const networkPolicyGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "NetworkPolicy Get Parameters",
  "description": "Parameters for retrieving a specific NetworkPolicy.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the NetworkPolicy."
    },
    "name": {
      "type": "string",
      "description": "Name of the NetworkPolicy."
    }
  },
  "required": ["namespace", "name"]
}`

type networkPolicyDetail struct {
	Name        string                    `json:"name,omitempty"`
	Namespace   string                    `json:"namespace,omitempty"`
	PodSelector map[string]string         `json:"pod_selector,omitempty"`
	PolicyTypes []string                  `json:"policy_types,omitempty"`
	Ingress     []networkPolicyRuleDetail `json:"ingress,omitempty"`
	Egress      []networkPolicyRuleDetail `json:"egress,omitempty"`
	Age         string                    `json:"age,omitempty"`
}

type networkPolicyRuleDetail struct {
	From  []networkPolicyPeerDetail `json:"from,omitempty"`
	To    []networkPolicyPeerDetail `json:"to,omitempty"`
	Ports []networkPolicyPortDetail `json:"ports,omitempty"`
}

type networkPolicyPeerDetail struct {
	PodSelector       map[string]string `json:"pod_selector,omitempty"`
	NamespaceSelector map[string]string `json:"namespace_selector,omitempty"`
	CIDRBlocks        []string          `json:"cidr_blocks,omitempty"`
}

type networkPolicyPortDetail struct {
	Protocol string `json:"protocol,omitempty"`
	Port     string `json:"port,omitempty"`
}
