package kubernetes

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointsListTask(t *testing.T) {
	t.Parallel()

	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
		Subsets: []corev1.EndpointSubset{{
			Addresses:         []corev1.EndpointAddress{{IP: "10.0.0.1"}, {IP: "10.0.0.2"}},
			NotReadyAddresses: []corev1.EndpointAddress{{IP: "10.0.0.3"}},
			Ports:             []corev1.EndpointPort{{Name: "http", Port: 8080, Protocol: corev1.ProtocolTCP}},
		}},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(endpoints)})
	task := &endpointsListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	items := data["endpoints"].([]endpointsSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "api", items[0].Name)
	assert.Equal(t, "default", items[0].Namespace)
	assert.NotEmpty(t, items[0].Age)
	require.Len(t, items[0].Subsets, 1)
	assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, items[0].Subsets[0].Addresses)
	assert.Equal(t, []string{"10.0.0.3"}, items[0].Subsets[0].NotReadyAddresses)
	require.Len(t, items[0].Subsets[0].Ports, 1)
	assert.Equal(t, int32(8080), items[0].Subsets[0].Ports[0].Port)
	assert.Equal(t, "TCP", items[0].Subsets[0].Ports[0].Protocol)
}

func TestEndpointsListTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &endpointsListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"limit": "notanint"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit")
}

func TestEndpointsListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &endpointsListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestEndpointsListTaskAllNamespaces(t *testing.T) {
	t.Parallel()

	endpoints1 := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}}
	endpoints2 := &corev1.Endpoints{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns2"}}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(endpoints1, endpoints2)})
	task := &endpointsListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}
