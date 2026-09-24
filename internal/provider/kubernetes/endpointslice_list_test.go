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

func TestEndpointSliceListTask(t *testing.T) {
	t.Parallel()

	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "nginx-abc",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			Labels: map[string]string{
				discoveryv1.LabelServiceName: "nginx",
			},
		},
		Ports: []discoveryv1.EndpointPort{{
			Name:     endpointSliceStringPtr("http"),
			Port:     endpointSliceInt32Ptr(80),
			Protocol: endpointSliceProtocolPtr(corev1.ProtocolTCP),
		}},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses: []string{"10.0.0.10", "fd00::1"},
			Conditions: discoveryv1.EndpointConditions{
				Ready:       endpointSliceBoolPtr(true),
				Serving:     endpointSliceBoolPtr(true),
				Terminating: endpointSliceBoolPtr(false),
			},
			NodeName: endpointSliceStringPtr("node-1"),
		}},
	}

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(endpointSlice)})
	task := &endpointSliceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	items := data["endpoint_slices"].([]endpointSliceSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "nginx-abc", items[0].Name)
	assert.Equal(t, "default", items[0].Namespace)
	assert.Equal(t, "nginx", items[0].ServiceName)
	require.Len(t, items[0].Ports, 1)
	assert.Equal(t, "http", items[0].Ports[0].Name)
	assert.Equal(t, int32(80), items[0].Ports[0].Port)
	assert.Equal(t, "TCP", items[0].Ports[0].Protocol)
	require.Len(t, items[0].Endpoints, 1)
	assert.Equal(t, "10.0.0.10", items[0].Endpoints[0].Address)
	assert.True(t, items[0].Endpoints[0].Ready)
	assert.True(t, items[0].Endpoints[0].Serving)
	assert.False(t, items[0].Endpoints[0].Terminating)
	assert.Equal(t, "node-1", items[0].Endpoints[0].NodeName)
	assert.NotEmpty(t, items[0].Age)
}

func TestEndpointSliceListTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &endpointSliceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"limit": "notanint"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "limit")
}

func TestEndpointSliceListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &endpointSliceListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestEndpointSliceListTaskAllNamespaces(t *testing.T) {
	t.Parallel()

	endpointSliceOne := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}}
	endpointSliceTwo := &discoveryv1.EndpointSlice{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns2"}}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(endpointSliceOne, endpointSliceTwo)})
	task := &endpointSliceListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}

func endpointSliceStringPtr(value string) *string { return &value }

func endpointSliceInt32Ptr(value int32) *int32 { return &value }

func endpointSliceBoolPtr(value bool) *bool { return &value }

func endpointSliceProtocolPtr(value corev1.Protocol) *corev1.Protocol { return &value }
