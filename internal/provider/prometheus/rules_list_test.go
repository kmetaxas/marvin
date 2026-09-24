package prometheus

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRulesListTaskNameAndSchema(t *testing.T) {
	t.Parallel()

	task := &rulesListTask{}
	assert.Equal(t, "prometheus.rules.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestRulesListTaskValidationErrors(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{guardrails: DefaultGuardrailPolicy()}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"type invalid", map[string]any{"type": "bogus"}, "type must be one of"},
		{"type wrong type", map[string]any{"type": 1}, "type must be a string"},
		{"rule_group wrong type", map[string]any{"rule_group": 1}, "rule_group must be a string"},
		{"file wrong type", map[string]any{"file": 1}, "file must be a string"},
		{"match_labels wrong type", map[string]any{"match_labels": "x"}, "match_labels must be a map of strings"},
		{"limit zero", map[string]any{"limit": 0}, "limit must be greater than 0"},
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

func TestRulesListTaskNilClient(t *testing.T) {
	t.Parallel()

	task := &rulesListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestRulesListTaskSuccess(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		rulesFunc: func(ctx context.Context, matches []string) (v1.RulesResult, error) {
			return v1.RulesResult{
				Groups: []v1.RuleGroup{
					{
						Name:     "group-a",
						File:     "/etc/prometheus/rules.yml",
						Interval: 30,
						Rules: v1.Rules{
							v1.AlertingRule{
								Name:        "HighCPU",
								Query:       "cpu > 0.9",
								Duration:    300,
								Labels:      model.LabelSet{"severity": "critical"},
								Annotations: model.LabelSet{"summary": "CPU high"},
								Health:      v1.RuleHealthGood,
								State:       "firing",
							},
							v1.RecordingRule{
								Name:   "job:up:sum",
								Query:  "sum(up) by (job)",
								Labels: model.LabelSet{"job": "prometheus"},
								Health: v1.RuleHealthGood,
							},
						},
					},
				},
			}, nil
		},
	}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, false, data["truncated"])
	assert.Equal(t, 2, data["count"])

	rules := data["rules"].([]map[string]any)
	require.Len(t, rules, 2)

	assert.Equal(t, "HighCPU", rules[0]["name"])
	assert.Equal(t, "alert", rules[0]["type"])
	assert.Equal(t, "group-a", rules[0]["rule_group"])
	assert.Equal(t, "/etc/prometheus/rules.yml", rules[0]["file"])
	assert.Equal(t, float64(30), rules[0]["interval"])
	assert.Equal(t, "critical", rules[0]["labels"].(map[string]string)["severity"])

	assert.Equal(t, "job:up:sum", rules[1]["name"])
	assert.Equal(t, "record", rules[1]["type"])
}

func TestRulesListTaskTypeFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		rulesFunc: func(ctx context.Context, matches []string) (v1.RulesResult, error) {
			return v1.RulesResult{
				Groups: []v1.RuleGroup{
					{
						Name: "g",
						Rules: v1.Rules{
							v1.AlertingRule{Name: "AlertA"},
							v1.RecordingRule{Name: "RecordB"},
						},
					},
				},
			}, nil
		},
	}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"type": "record"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	rules := data["rules"].([]map[string]any)
	require.Len(t, rules, 1)
	assert.Equal(t, "RecordB", rules[0]["name"])
}

func TestRulesListTaskGroupAndFileFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		rulesFunc: func(ctx context.Context, matches []string) (v1.RulesResult, error) {
			return v1.RulesResult{
				Groups: []v1.RuleGroup{
					{Name: "keep", File: "a.yml", Rules: v1.Rules{v1.AlertingRule{Name: "A"}}},
					{Name: "drop", File: "b.yml", Rules: v1.Rules{v1.AlertingRule{Name: "B"}}},
				},
			}, nil
		},
	}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"rule_group": "keep", "file": "a.yml"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	rules := data["rules"].([]map[string]any)
	require.Len(t, rules, 1)
	assert.Equal(t, "A", rules[0]["name"])
}

func TestRulesListTaskMatchLabelsFilter(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		rulesFunc: func(ctx context.Context, matches []string) (v1.RulesResult, error) {
			return v1.RulesResult{
				Groups: []v1.RuleGroup{
					{
						Name: "g",
						Rules: v1.Rules{
							v1.AlertingRule{Name: "A", Labels: model.LabelSet{"severity": "critical"}},
							v1.AlertingRule{Name: "B", Labels: model.LabelSet{"severity": "warning"}},
						},
					},
				},
			}, nil
		},
	}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{"match_labels": map[string]string{"severity": "critical"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	rules := data["rules"].([]map[string]any)
	require.Len(t, rules, 1)
	assert.Equal(t, "A", rules[0]["name"])
}

func TestRulesListTaskTruncation(t *testing.T) {
	t.Parallel()

	guardrails := DefaultGuardrailPolicy()
	guardrails.MaxReturnedSeries = 2

	client := &fakePrometheusClient{
		guardrails: guardrails,
		rulesFunc: func(ctx context.Context, matches []string) (v1.RulesResult, error) {
			return v1.RulesResult{
				Groups: []v1.RuleGroup{
					{
						Name: "g",
						Rules: v1.Rules{
							v1.AlertingRule{Name: "A"},
							v1.AlertingRule{Name: "B"},
							v1.AlertingRule{Name: "C"},
						},
					},
				},
			}, nil
		},
	}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, true, data["truncated"])
	assert.Equal(t, 2, data["count"])
	rules := data["rules"].([]map[string]any)
	assert.Len(t, rules, 2)
}

func TestRulesListTaskAPIError(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		rulesFunc: func(ctx context.Context, matches []string) (v1.RulesResult, error) {
			return v1.RulesResult{}, errors.New("api failed")
		},
	}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api failed")
}

func TestRulesListTaskContextCancellation(t *testing.T) {
	t.Parallel()

	client := &fakePrometheusClient{
		guardrails: DefaultGuardrailPolicy(),
		rulesFunc: func(ctx context.Context, matches []string) (v1.RulesResult, error) {
			return v1.RulesResult{}, context.Canceled
		},
	}
	task := &rulesListTask{provider: fakeClientProvider(client)}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := task.Execute(ctx, map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "canceled")
}
