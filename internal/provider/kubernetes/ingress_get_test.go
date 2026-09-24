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

func TestIngressGetTask(t *testing.T) {
	t.Parallel()

	className := "nginx"
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "web",
			Namespace:         "default",
			CreationTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &className,
			Rules: []networkingv1.IngressRule{{
				Host: "example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: "nginx",
						Port: networkingv1.ServiceBackendPort{Number: 80},
					}},
				}}}},
			}},
			TLS: []networkingv1.IngressTLS{{Hosts: []string{"example.com"}, SecretName: "web-tls"}},
		},
		Status: networkingv1.IngressStatus{LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: []networkingv1.IngressLoadBalancerIngress{{Hostname: "lb.example.com"}}}},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ingress)})
	task := &ingressGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "web"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	item := data["ingress"].(ingressDetail)
	assert.Equal(t, "web", item.Name)
	assert.Equal(t, "default", item.Namespace)
	assert.Equal(t, "nginx", item.Class)
	require.Len(t, item.Rules, 1)
	assert.Equal(t, "example.com", item.Rules[0].Host)
	require.Len(t, item.Rules[0].Paths, 1)
	assert.Equal(t, "/", item.Rules[0].Paths[0].Path)
	assert.Equal(t, string(pathType), item.Rules[0].Paths[0].PathType)
	assert.Equal(t, "Service/nginx:80", item.Rules[0].Paths[0].Backend)
	require.Len(t, item.TLS, 1)
	assert.Equal(t, []string{"example.com"}, item.TLS[0].Hosts)
	assert.Equal(t, "web-tls", item.TLS[0].SecretName)
	assert.Equal(t, "lb.example.com", item.Status.LoadBalancer)
	assert.NotEmpty(t, item.Age)
}

func TestIngressGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &ingressGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestIngressGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &ingressGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "web"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestIngressGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &ingressGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
