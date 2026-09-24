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

func TestEndpointsGetTask(t *testing.T) {
	t.Parallel()

	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "api",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-30 * time.Minute)),
		},
		Subsets: []corev1.EndpointSubset{{
			Addresses:         []corev1.EndpointAddress{{IP: "10.0.0.1"}},
			NotReadyAddresses: []corev1.EndpointAddress{{IP: "10.0.0.2"}},
			Ports:             []corev1.EndpointPort{{Name: "http", Port: 8080, Protocol: corev1.ProtocolTCP}},
		}},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(endpoints)})
	task := &endpointsGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	item := data["endpoints"].(endpointsDetail)
	assert.Equal(t, "api", item.Name)
	assert.Equal(t, "default", item.Namespace)
	assert.NotEmpty(t, item.Age)
	require.Len(t, item.Subsets, 1)
	assert.Equal(t, []string{"10.0.0.1"}, item.Subsets[0].Addresses)
	assert.Equal(t, []string{"10.0.0.2"}, item.Subsets[0].NotReadyAddresses)
	require.Len(t, item.Subsets[0].Ports, 1)
	assert.Equal(t, int32(8080), item.Subsets[0].Ports[0].Port)
	assert.Equal(t, "TCP", item.Subsets[0].Ports[0].Protocol)
}

func TestEndpointsGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &endpointsGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestEndpointsGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &endpointsGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestEndpointsGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &endpointsGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
