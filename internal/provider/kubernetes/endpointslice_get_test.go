package kubernetes

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointSliceGetTask(t *testing.T) {
	t.Parallel()

	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "nginx-abc",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-90 * time.Minute)),
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "nginx",
			},
		},
		Ports: []discoveryv1.EndpointPort{{
			Name:     endpointSliceStringPtr("https"),
			Port:     endpointSliceInt32Ptr(443),
			Protocol: endpointSliceProtocolPtr(corev1.ProtocolTCP),
		}},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses: []string{"10.0.0.11"},
			Conditions: discoveryv1.EndpointConditions{
				Ready:       endpointSliceBoolPtr(true),
				Serving:     endpointSliceBoolPtr(true),
				Terminating: endpointSliceBoolPtr(false),
			},
			NodeName: endpointSliceStringPtr("node-2"),
		}},
	}

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(endpointSlice)})
	task := &endpointSliceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "nginx-abc"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	item := data["endpoint_slice"].(endpointSliceDetail)
	assert.Equal(t, "nginx-abc", item.Name)
	assert.Equal(t, "default", item.Namespace)
	assert.Equal(t, "nginx", item.ServiceName)
	require.Len(t, item.Ports, 1)
	assert.Equal(t, "https", item.Ports[0].Name)
	assert.Equal(t, int32(443), item.Ports[0].Port)
	assert.Equal(t, "TCP", item.Ports[0].Protocol)
	require.Len(t, item.Endpoints, 1)
	assert.Equal(t, "10.0.0.11", item.Endpoints[0].Address)
	assert.Equal(t, "node-2", item.Endpoints[0].NodeName)
	assert.NotEmpty(t, item.Age)
}

func TestEndpointSliceGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &endpointSliceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestEndpointSliceGetTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &endpointSliceGetTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "nginx-abc"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestEndpointSliceGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &endpointSliceGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
