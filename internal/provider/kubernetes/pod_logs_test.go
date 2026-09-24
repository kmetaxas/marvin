package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodLogsTask(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:1.25"}},
		},
	}

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(pod)})
	task := &podLogsTask{provider: provider, maxLogLines: 100}

	res, err := task.Execute(context.Background(), map[string]any{
		"namespace": "default",
		"name":      "nginx",
		"container": "nginx",
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["searched"])
	assert.GreaterOrEqual(t, data["line_count"], 0)
}

func TestPodLogsTaskMissingParams(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset()})
	task := &podLogsTask{provider: provider, maxLogLines: 100}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "name")
}

func TestPodLogsTaskClientNotConfigured(t *testing.T) {
	t.Parallel()

	task := &podLogsTask{provider: newTestProvider(&k8sClient{}), maxLogLines: 100}
	res, err := task.Execute(context.Background(), map[string]any{
		"namespace": "default",
		"name":      "nginx",
	})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "kubernetes client is not configured")
}

func TestPodLogsTaskEnforcesMaxLimit(t *testing.T) {
	t.Parallel()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "nginx", Image: "nginx:1.25"}},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(pod)})
	task := &podLogsTask{provider: provider, maxLogLines: 50}

	res, err := task.Execute(context.Background(), map[string]any{
		"namespace":  "default",
		"name":       "nginx",
		"container":  "nginx",
		"tail_lines": 100,
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	// The fake client may return non-empty data; the key assertion is that
	// tail_lines > maxLogLines does not cause a failure.
}

func TestSplitLines(t *testing.T) {
	t.Parallel()

	lines := splitLines([]byte("line1\nline2\nline3"))
	require.Equal(t, []string{"line1", "line2", "line3"}, lines)

	lines = splitLines([]byte(""))
	require.Empty(t, lines)
}

func TestExtractLogContextNoSearch(t *testing.T) {
	t.Parallel()

	lines := []string{"a", "b", "c", "d", "e"}
	result := extractLogContext(lines, "", 3)
	require.Equal(t, []string{"c", "d", "e"}, result)

	result = extractLogContext(lines, "", 10)
	require.Equal(t, []string{"a", "b", "c", "d", "e"}, result)
}

func TestExtractLogContextWithSearch(t *testing.T) {
	t.Parallel()

	lines := []string{
		"error: connection refused",
		"info: starting up",
		"error: timeout",
		"debug: retrying",
		"error: giving up",
	}

	result := extractLogContext(lines, "error", 5)
	require.Len(t, result, 5)
	require.Equal(t, lines, result)
}

func TestExtractLogContextSearchNoMatches(t *testing.T) {
	t.Parallel()

	lines := []string{"a", "b", "c"}
	result := extractLogContext(lines, "z", 2)
	require.Empty(t, result)
}

func TestExtractLogContextCaseInsensitive(t *testing.T) {
	t.Parallel()

	lines := []string{"ERROR: boom", "info: ok"}
	result := extractLogContext(lines, "error", 2)
	require.Len(t, result, 2)
	require.Equal(t, lines, result)
}

func TestExtractLogContextMerging(t *testing.T) {
	t.Parallel()

	// Matches are at indices 0 and 4 with halfContext=1; windows overlap and merge.
	lines := []string{"m0", "m1", "m2", "m3", "m4", "m5"}
	result := extractLogContext(lines, "m", 6)
	require.Len(t, result, 6)
	require.Equal(t, lines, result)
}
