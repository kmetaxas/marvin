package kubernetes

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceGetTask(t *testing.T) {
	t.Parallel()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour)),
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeLoadBalancer,
			ClusterIP:             "10.96.0.10",
			ExternalIPs:           []string{"1.2.3.4"},
			Selector:              map[string]string{"app": "api"},
			SessionAffinity:       corev1.ServiceAffinityClientIP,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080), Protocol: corev1.ProtocolTCP},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "34.1.2.3"}, {IP: "34.1.2.4"}},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(svc)})
	task := &serviceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	service := data["service"].(serviceDetail)
	assert.Equal(t, "api", service.Name)
	assert.Equal(t, "default", service.Namespace)
	assert.Equal(t, "LoadBalancer", service.Type)
	assert.Equal(t, "10.96.0.10", service.ClusterIP)
	assert.Equal(t, []string{"1.2.3.4"}, service.ExternalIPs)
	assert.Equal(t, []string{"34.1.2.3", "34.1.2.4"}, service.LoadBalancerIPs)
	assert.Equal(t, "ClientIP", service.SessionAffinity)
	assert.Equal(t, "Local", service.ExternalTrafficPolicy)
	assert.NotEmpty(t, service.Age)
	require.Len(t, service.Ports, 1)
	assert.Equal(t, "8080", service.Ports[0].TargetPort)
}

func TestServiceGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &serviceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestServiceGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &serviceGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestServiceGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &serviceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
