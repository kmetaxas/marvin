package kubernetes

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkPolicyGetTask(t *testing.T) {
	t.Parallel()

	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web-allow",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-4 * time.Hour)),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"role": "web"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "edge"}},
					},
					{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/24", Except: []string{"10.0.0.10/32"}}},
				},
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: networkPolicyProtocolPtr(corev1.ProtocolTCP),
					Port:     networkPolicyIntOrStringPtr(intstr.FromInt(80)),
				}},
			}},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"name": "shared"}},
				}},
				Ports: []networkingv1.NetworkPolicyPort{{
					Protocol: networkPolicyProtocolPtr(corev1.ProtocolUDP),
					Port:     networkPolicyIntOrStringPtr(intstr.FromString("dns")),
				}},
			}},
		},
	}

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(networkPolicy)})
	task := &networkPolicyGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "web-allow"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	item := data["network_policy"].(networkPolicyDetail)
	assert.Equal(t, "web-allow", item.Name)
	assert.Equal(t, "default", item.Namespace)
	assert.Equal(t, map[string]string{"role": "web"}, item.PodSelector)
	assert.Equal(t, []string{"Ingress", "Egress"}, item.PolicyTypes)
	require.Len(t, item.Ingress, 1)
	require.Len(t, item.Ingress[0].From, 2)
	assert.Equal(t, map[string]string{"app": "frontend"}, item.Ingress[0].From[0].PodSelector)
	assert.Equal(t, map[string]string{"team": "edge"}, item.Ingress[0].From[0].NamespaceSelector)
	assert.Empty(t, item.Ingress[0].From[0].CIDRBlocks)
	assert.Equal(t, map[string]string{}, item.Ingress[0].From[1].PodSelector)
	assert.Equal(t, map[string]string{}, item.Ingress[0].From[1].NamespaceSelector)
	assert.Equal(t, []string{"10.0.0.0/24", "10.0.0.10/32"}, item.Ingress[0].From[1].CIDRBlocks)
	require.Len(t, item.Ingress[0].Ports, 1)
	assert.Equal(t, "TCP", item.Ingress[0].Ports[0].Protocol)
	assert.Equal(t, "80", item.Ingress[0].Ports[0].Port)
	require.Len(t, item.Egress, 1)
	require.Len(t, item.Egress[0].To, 1)
	assert.Equal(t, map[string]string{}, item.Egress[0].To[0].PodSelector)
	assert.Equal(t, map[string]string{"name": "shared"}, item.Egress[0].To[0].NamespaceSelector)
	require.Len(t, item.Egress[0].Ports, 1)
	assert.Equal(t, "UDP", item.Egress[0].Ports[0].Protocol)
	assert.Equal(t, "dns", item.Egress[0].Ports[0].Port)
	assert.NotEmpty(t, item.Age)
}

func TestNetworkPolicyGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &networkPolicyGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestNetworkPolicyGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &networkPolicyGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "web-allow"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestNetworkPolicyGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &networkPolicyGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}

func networkPolicyProtocolPtr(value corev1.Protocol) *corev1.Protocol { return &value }

func networkPolicyIntOrStringPtr(value intstr.IntOrString) *intstr.IntOrString { return &value }
