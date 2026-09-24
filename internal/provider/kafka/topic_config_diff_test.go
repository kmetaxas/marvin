package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestTopicConfigDiffTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &topicConfigDiffTask{}
	assert.Equal(t, "kafka.topic.config.diff", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTopicConfigDiffTaskMissingTopics(t *testing.T) {
	t.Parallel()
	task := &topicConfigDiffTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "topics")
}

func TestTopicConfigDiffTaskDetectsAnomaly(t *testing.T) {
	t.Parallel()
	ordersRetention := "604800000"
	paymentsRetention := "86400000"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeTopicConfigsFunc: func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name:    "orders",
					Configs: []kadm.Config{{Key: "retention.ms", Value: &ordersRetention, Source: kmsg.ConfigSourceDynamicTopicConfig}},
				},
				{
					Name:    "payments",
					Configs: []kadm.Config{{Key: "retention.ms", Value: &paymentsRetention, Source: kmsg.ConfigSourceDynamicTopicConfig}},
				},
			}, nil
		},
	}
	task := &topicConfigDiffTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topics": []any{"orders", "payments"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	diff := data["diff"].([]map[string]any)
	require.Len(t, diff, 1)
	assert.Equal(t, "retention.ms", diff[0]["key"])
	anomalous := diff[0]["anomalous_topics"].([]string)
	assert.Equal(t, []string{"payments"}, anomalous)
}

func TestTopicConfigDiffTaskNoDiff(t *testing.T) {
	t.Parallel()
	retention := "604800000"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeTopicConfigsFunc: func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name:    "orders",
					Configs: []kadm.Config{{Key: "retention.ms", Value: &retention, Source: kmsg.ConfigSourceDynamicTopicConfig}},
				},
				{
					Name:    "payments",
					Configs: []kadm.Config{{Key: "retention.ms", Value: &retention, Source: kmsg.ConfigSourceDynamicTopicConfig}},
				},
			}, nil
		},
	}
	task := &topicConfigDiffTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topics": []any{"orders", "payments"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
}
