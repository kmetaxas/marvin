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

func TestEventListTask(t *testing.T) {
	t.Parallel()

	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "nginx.123abc", Namespace: "default"},
		Type:           "Warning",
		Reason:         "FailedScheduling",
		Message:        "0/3 nodes are available",
		Source:         corev1.EventSource{Component: "default-scheduler"},
		FirstTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
		LastTimestamp:  metav1.Time{Time: time.Now()},
		Count:          5,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "nginx-abc123"},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ev)})
	task := &eventListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	items := data["events"].([]eventSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "nginx.123abc", items[0].Name)
	assert.Equal(t, "Warning", items[0].Type)
	assert.Equal(t, "FailedScheduling", items[0].Reason)
	assert.Equal(t, "default-scheduler", items[0].Source)
	assert.Equal(t, 5, items[0].Count)
	assert.Equal(t, "Pod/nginx-abc123", items[0].InvolvedObject)
}

func TestEventGetTask(t *testing.T) {
	t.Parallel()

	ev := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "nginx.123abc", Namespace: "default"},
		Type:           "Warning",
		Reason:         "FailedScheduling",
		Message:        "0/3 nodes are available",
		Source:         corev1.EventSource{Component: "default-scheduler"},
		FirstTimestamp: metav1.Time{Time: time.Now().Add(-time.Hour)},
		LastTimestamp:  metav1.Time{Time: time.Now()},
		Count:          5,
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "nginx-abc123"},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(ev)})
	task := &eventGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "nginx.123abc"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	e := data["event"].(eventSummary)
	assert.Equal(t, "nginx.123abc", e.Name)
	assert.Equal(t, "Warning", e.Type)
	assert.Equal(t, "FailedScheduling", e.Reason)
	assert.Equal(t, "default-scheduler", e.Source)
	assert.Equal(t, 5, e.Count)
	assert.Equal(t, "Pod/nginx-abc123", e.InvolvedObject)
}
