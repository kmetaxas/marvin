package kubernetes

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobListTask(t *testing.T) {
	t.Parallel()

	completions := int32(1)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-240101", Namespace: "default"},
		Spec:       batchv1.JobSpec{Completions: &completions},
		Status:     batchv1.JobStatus{Succeeded: 1},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(job)})
	task := &jobListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	items := data["jobs"].([]jobSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "backup-240101", items[0].Name)
	assert.Equal(t, "1/1", items[0].Completions)
}

func TestJobGetTask(t *testing.T) {
	t.Parallel()

	completions := int32(1)
	startTime := metav1.Time{Time: time.Now().Add(-time.Hour)}
	completionTime := metav1.Time{Time: time.Now().Add(-time.Minute)}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-240101", Namespace: "default", Labels: map[string]string{"app": "backup"}},
		Spec:       batchv1.JobSpec{Completions: &completions},
		Status: batchv1.JobStatus{
			Succeeded:      1,
			StartTime:      &startTime,
			CompletionTime: &completionTime,
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(job)})
	task := &jobGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "backup-240101"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	j := data["job"].(jobDetail)
	assert.Equal(t, "backup-240101", j.Name)
	assert.Equal(t, "1/1", j.Completions)
	assert.NotEmpty(t, j.Duration)
	require.Len(t, j.Conditions, 1)
	assert.Equal(t, "Complete", j.Conditions[0].Type)
	assert.Equal(t, "True", j.Conditions[0].Status)
	assert.Equal(t, map[string]string{"app": "backup"}, j.Labels)
}

func TestCronJobListTask(t *testing.T) {
	t.Parallel()

	suspend := false
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 2 * * *",
			Suspend:  &suspend,
		},
		Status: batchv1.CronJobStatus{Active: []corev1.ObjectReference{}},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(cj)})
	task := &cronJobListTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	items := data["cronjobs"].([]cronJobSummary)
	require.Len(t, items, 1)
	assert.Equal(t, "backup", items[0].Name)
	assert.Equal(t, "0 2 * * *", items[0].Schedule)
	assert.False(t, items[0].Suspend)
	assert.Equal(t, 0, items[0].Active)
}

func TestCronJobGetTask(t *testing.T) {
	t.Parallel()

	suspend := false
	lastSchedule := metav1.Time{Time: time.Now().Add(-time.Hour)}
	lastSuccessful := metav1.Time{Time: time.Now().Add(-time.Minute)}
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default", Labels: map[string]string{"app": "backup"}},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 2 * * *",
			Suspend:  &suspend,
		},
		Status: batchv1.CronJobStatus{
			Active:             []corev1.ObjectReference{},
			LastScheduleTime:   &lastSchedule,
			LastSuccessfulTime: &lastSuccessful,
		},
	}
	provider := newTestProvider(&k8sClient{clientset: fake.NewSimpleClientset(cj)})
	task := &cronJobGetTask{provider: provider}

	res, err := task.Execute(context.Background(), map[string]any{"namespace": "default", "name": "backup"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	data := res.Data.(map[string]any)
	c := data["cronjob"].(cronJobDetail)
	assert.Equal(t, "backup", c.Name)
	assert.Equal(t, "0 2 * * *", c.Schedule)
	assert.False(t, c.Suspend)
	assert.NotEmpty(t, c.LastScheduleTime)
	assert.NotEmpty(t, c.LastSuccessfulTime)
	assert.Equal(t, map[string]string{"app": "backup"}, c.Labels)
}
