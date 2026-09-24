package kubernetes

import (
	"context"
	"testing"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkPolicyListTask(t *testing.T) {
	t.Parallel()

	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deny-all",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-3 * time.Hour)),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "nginx"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{}},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{}, {}},
		},
	}

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(networkPolicy)})
	task := &networkPolicyListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	items := data["network_policies"].([]networkPolicySummary)
	require.Len(t, items, 1)
	assert.Equal(t, "deny-all", items[0].Name)
	assert.Equal(t, "default", items[0].Namespace)
	assert.Equal(t, map[string]string{"app": "nginx"}, items[0].PodSelector)
	assert.Equal(t, []string{"Ingress", "Egress"}, items[0].PolicyTypes)
	assert.Equal(t, 1, items[0].IngressRulesCount)
	assert.Equal(t, 2, items[0].EgressRulesCount)
	assert.NotEmpty(t, items[0].Age)
}

func TestNetworkPolicyListTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &networkPolicyListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"limit": "notanint"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit")
}

func TestNetworkPolicyListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &networkPolicyListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestNetworkPolicyListTaskAllNamespaces(t *testing.T) {
	t.Parallel()

	networkPolicyOne := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}}
	networkPolicyTwo := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns2"}}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(networkPolicyOne, networkPolicyTwo)})
	task := &networkPolicyListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}
