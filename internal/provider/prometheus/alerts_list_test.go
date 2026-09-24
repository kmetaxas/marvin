package prometheus

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlertsListTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &alertsListTask{}
	assert.Equal(t, "prometheus.alerts.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestAlertsListTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &alertsListTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"state invalid", map[string]any{"state": "bogus"}, "state must be one of"},
		{"state wrong type", map[string]any{"state": 1}, "state must be a string"},
		{"match_labels wrong type", map[string]any{"match_labels": "x"}, "match_labels must be a map of strings"},
		{"match_labels non-string value", map[string]any{"match_labels": map[string]any{"a": 1}}, "match_labels must be a map of strings"},
		{"limit zero", map[string]any{"limit": 0}, "limit must be greater than 0"},
		{"limit wrong type", map[string]any{"limit": "10"}, "limit must be an integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := task.Execute(context.Background(), tt.params)
			require.NoError(t, err)
			assert.False(t, res.Success)
			assert.Contains(t, res.Error, tt.want)
		})
	}
}

func TestAlertsListTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &alertsListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestAlertsListTaskSuccess(t *testing.T) {
	t.Parallel()

	activeAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		alertsFunc: func(ctx context.Context) (v1.AlertsResult, error) {
			return v1.AlertsResult{
				Alerts: []v1.Alert{
					{
						ActiveAt:    activeAt,
						Labels:      model.LabelSet{"alertname": "HighCPU", "severity": "critical"},
						Annotations: model.LabelSet{"summary": "CPU high"},
						State:       v1.AlertStateFiring,
						Value:       "0.95",
					},
					{
						ActiveAt:    activeAt,
						Labels:      model.LabelSet{"alertname": "LowDisk", "severity": "warning"},
						Annotations: model.LabelSet{"summary": "Disk low"},
						State:       v1.AlertStatePending,
						Value:       "0.10",
					},
				},
			}, nil
		},
	}
	task := &alertsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 2, data["count"])

	alerts := data["alerts"].([]map[string]any)
	require.Len(t, alerts, 2)
	assert.Equal(t, "HighCPU", alerts[0]["name"])
	assert.Equal(t, "firing", alerts[0]["state"])
	assert.Equal(t, "0.95", alerts[0]["value"])
	assert.Equal(t, "2024-01-15T10:30:00Z", alerts[0]["active_at"])
	assert.Equal(t, "critical", alerts[0]["labels"].(map[string]string)["severity"])
	assert.Equal(t, "CPU high", alerts[0]["annotations"].(map[string]string)["summary"])
}

func TestAlertsListTaskStateFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		alertsFunc: func(ctx context.Context) (v1.AlertsResult, error) {
			return v1.AlertsResult{
				Alerts: []v1.Alert{
					{Labels: model.LabelSet{"alertname": "A"}, State: v1.AlertStateFiring},
					{Labels: model.LabelSet{"alertname": "B"}, State: v1.AlertStatePending},
				},
			}, nil
		},
	}
	task := &alertsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"state": "firing"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	alerts := data["alerts"].([]map[string]any)
	require.Len(t, alerts, 1)
	assert.Equal(t, "A", alerts[0]["name"])
}

func TestAlertsListTaskMatchLabelsFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		alertsFunc: func(ctx context.Context) (v1.AlertsResult, error) {
			return v1.AlertsResult{
				Alerts: []v1.Alert{
					{Labels: model.LabelSet{"alertname": "A", "severity": "critical"}, State: v1.AlertStateFiring},
					{Labels: model.LabelSet{"alertname": "B", "severity": "warning"}, State: v1.AlertStateFiring},
				},
			}, nil
		},
	}
	task := &alertsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"match_labels": map[string]string{"severity": "critical"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	alerts := data["alerts"].([]map[string]any)
	require.Len(t, alerts, 1)
	assert.Equal(t, "A", alerts[0]["name"])
}

func TestAlertsListTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxReturnedSeries = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		alertsFunc: func(ctx context.Context) (v1.AlertsResult, error) {
			return v1.AlertsResult{
				Alerts: []v1.Alert{
					{Labels: model.LabelSet{"alertname": "A"}, State: v1.AlertStateFiring},
					{Labels: model.LabelSet{"alertname": "B"}, State: v1.AlertStateFiring},
					{Labels: model.LabelSet{"alertname": "C"}, State: v1.AlertStateFiring},
				},
			}, nil
		},
	}
	task := &alertsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	alerts := data["alerts"].([]map[string]any)
	assert.Len(t, alerts, 2)
}

func TestAlertsListTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		alertsFunc: func(ctx context.Context) (v1.AlertsResult, error) {
			return v1.AlertsResult{}, errors.New("api failed")
		},
	}
	task := &alertsListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestAlertsListTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		alertsFunc: func(ctx context.Context) (v1.AlertsResult, error) {
			return v1.AlertsResult{}, context.Canceled
		},
	}
	task := &alertsListTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
