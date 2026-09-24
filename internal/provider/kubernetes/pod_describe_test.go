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

func TestPodDescribeTask(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(time.Now())
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default", CreationTimestamp: now, Labels: map[string]string{"app": "api"}},
		Spec: corev1.PodSpec{
			NodeName:           "node-1",
			ServiceAccountName: "default",
			RestartPolicy:      corev1.RestartPolicyAlways,
			Containers:         []corev1.Container{{Name: "api", Image: "nginx:latest"}},
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodRunning,
			PodIP:     "10.244.0.10",
			StartTime: &now,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
				Reason: "ContainersReady",
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "api",
				Ready:        true,
				RestartCount: 1,
				State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: now}},
			}},
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "api-started", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "api",
			Namespace: "default",
		},
		Type:           corev1.EventTypeNormal,
		Reason:         "Started",
		Message:        "Started container api",
		Count:          2,
		FirstTimestamp: now,
		LastTimestamp:  now,
		Source:         corev1.EventSource{Component: "kubelet"},
	}
	otherEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "other", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "other", Namespace: "default"},
		Reason:         "Ignored",
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(pod, event, otherEvent)})
	task := &podDescribeTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api", "event_limit": 5})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	detail := data["pod"].(podDetail)
	assert.Equal(t, "api", detail.Name)
	assert.Equal(t, "Running", detail.Phase)
	assert.Equal(t, "10.244.0.10", detail.PodIP)
	require.Len(t, detail.Containers, 1)
	assert.Equal(t, "running", detail.Containers[0].State)
	assert.Equal(t, 1, detail.Containers[0].RestartCount)
	assert.Equal(t, 1, data["event_count"])
	events := data["events"].([]eventSummary)
	require.Len(t, events, 1)
	assert.Equal(t, "Started", events[0].Reason)
	assert.Equal(t, "kubelet", events[0].Source)
}

func TestPodDescribeTaskMissingRequiredParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &podDescribeTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestPodDescribeTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &podDescribeTask{provider: newTestProvider(&k8sClient{})}
	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "api"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}
