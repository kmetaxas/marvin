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

func TestServiceListTask(t *testing.T) {
	t.Parallel()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
		Spec: corev1.ServiceSpec{
			Type:        corev1.ServiceTypeLoadBalancer,
			ClusterIP:   "10.96.0.10",
			ExternalIPs: []string{"1.2.3.4"},
			Selector:    map[string]string{"app": "api"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080), Protocol: corev1.ProtocolTCP},
				{Name: "https", Port: 443, TargetPort: intstr.FromString("https"), Protocol: corev1.ProtocolTCP},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(svc)})
	task := &serviceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	services := data["services"].([]serviceSummary)
	require.Len(t, services, 1)
	assert.Equal(t, "api", services[0].Name)
	assert.Equal(t, "default", services[0].Namespace)
	assert.Equal(t, "LoadBalancer", services[0].Type)
	assert.Equal(t, "10.96.0.10", services[0].ClusterIP)
	assert.Equal(t, []string{"1.2.3.4"}, services[0].ExternalIPs)
	assert.Equal(t, map[string]string{"app": "api"}, services[0].Selector)
	assert.NotEmpty(t, services[0].Age)
	require.Len(t, services[0].Ports, 2)
	assert.Equal(t, "8080", services[0].Ports[0].TargetPort)
	assert.Equal(t, "https", services[0].Ports[1].TargetPort)
}

func TestServiceListTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &serviceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"limit": "notanint"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit")
}

func TestServiceListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &serviceListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestServiceListTaskAllNamespaces(t *testing.T) {
	t.Parallel()

	svc1 := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}}
	svc2 := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns2"}}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(svc1, svc2)})
	task := &serviceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}

func TestServiceListTaskFiltersByType(t *testing.T) {
	t.Parallel()

	clusterIP := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "internal", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
	}
	loadBalancer := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "public", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(clusterIP, loadBalancer)})
	task := &serviceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "type": "LoadBalancer"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	services := data["services"].([]serviceSummary)
	require.Len(t, services, 1)
	assert.Equal(t, "public", services[0].Name)
	assert.Equal(t, "LoadBalancer", services[0].Type)
}

func TestServiceListTaskRejectsInvalidType(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &serviceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"type": "Invalid"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "parameter type")
}
