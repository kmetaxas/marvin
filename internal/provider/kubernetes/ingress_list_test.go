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

func TestIngressListTask(t *testing.T) {
	t.Parallel()

	className := "nginx"
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web",
			Namespace:         "default",
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules:            []networkingv1.IngressRule{{Host: "example.com"}, {Host: "api.example.com"}},
			TLS:              []networkingv1.IngressTLS{{Hosts: []string{"example.com", "api.example.com"}, SecretName: "web-tls"}},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ingress)})
	task := &ingressListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	items := data["ingresses"].([]ingressSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "web", items[0].Name)
	assert.Equal(t, "default", items[0].Namespace)
	assert.Equal(t, "nginx", items[0].Class)
	assert.Equal(t, []string{"example.com", "api.example.com"}, items[0].Hosts)
	assert.Equal(t, []string{"example.com", "api.example.com"}, items[0].TLSHosts)
	assert.Equal(t, 2, items[0].RulesCount)
	assert.NotEmpty(t, items[0].Age)
}

func TestIngressListTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &ingressListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"limit": "bad"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit")
}

func TestIngressListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &ingressListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestIngressListTaskAllNamespaces(t *testing.T) {
	t.Parallel()

	ingressA := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}}
	ingressB := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns2"}}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ingressA, ingressB)})
	task := &ingressListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}
