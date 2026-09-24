package kubernetes

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeListTask(t *testing.T) {
	t.Parallel()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.30.0",
				OSImage:                 "Ubuntu 22.04",
				KernelVersion:           "5.15.0",
				ContainerRuntimeVersion: "containerd://1.7.0",
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
				{Type: corev1.NodeExternalIP, Address: "203.0.113.1"},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(node)})
	task := &nodeListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	nodes := data["nodes"].([]nodeSummary)
	require.Len(t, nodes, 1)
	assert.Equal(t, "node-1", nodes[0].Name)
	assert.Equal(t, []string{"control-plane"}, nodes[0].Roles)
	assert.Equal(t, "Ready", nodes[0].Status)
	assert.Equal(t, "v1.30.0", nodes[0].Version)
	assert.Equal(t, "10.0.0.1", nodes[0].InternalIP)
	assert.Equal(t, "203.0.113.1", nodes[0].ExternalIP)
}

func TestNodeListTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &nodeListTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestNodeGetTask(t *testing.T) {
	t.Parallel()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{"node-role.kubernetes.io/worker": ""},
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue, Reason: "KubeletReady"},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
			},
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.30.0",
				OSImage:                 "Ubuntu 22.04",
				KernelVersion:           "5.15.0",
				ContainerRuntimeVersion: "containerd://1.7.0",
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    *(resource.NewQuantity(8, resource.DecimalSI)),
				corev1.ResourceMemory: *(resource.NewQuantity(32*1024*1024*1024, resource.BinarySI)),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    *(resource.NewQuantity(7, resource.DecimalSI)),
				corev1.ResourceMemory: *(resource.NewQuantity(30*1024*1024*1024, resource.BinarySI)),
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(node)})
	task := &nodeGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "node-1"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	n := data["node"].(nodeDetail)
	assert.Equal(t, "node-1", n.Name)
	assert.Equal(t, []string{"worker"}, n.Roles)
	assert.Equal(t, "Ready", n.Status)
	require.Len(t, n.Conditions, 2)
	assert.Equal(t, "Ready", n.Conditions[0].Type)
	assert.Equal(t, "True", n.Conditions[0].Status)
	assert.Equal(t, "KubeletReady", n.Conditions[0].Reason)
	require.Len(t, n.Taints, 1)
	assert.Equal(t, "NoSchedule", n.Taints[0].Effect)
	assert.NotEmpty(t, n.Capacity)
	assert.NotEmpty(t, n.Allocatable)
}

func TestNodeGetTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &nodeGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestNodeGetTaskNotFound(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &nodeGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"name": "missing"})
	require.NoError(t, err)
	assert.False(t, res.Success)
}
