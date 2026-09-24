package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestTopicConfigGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &topicConfigGetTask{}
	assert.Equal(t, "kafka.topic.config.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestTopicConfigGetTaskMissingTopic(t *testing.T) {
	t.Parallel()
	task := &topicConfigGetTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "topic")
}

func TestTopicConfigGetTaskSuccess(t *testing.T) {
	t.Parallel()
	retention := "604800000"
	cleanup := "delete"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeTopicConfigsFunc: func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name: "orders",
					Configs: []kadm.Config{
						{Key: "retention.ms", Value: &retention, Source: kmsg.ConfigSourceDynamicTopicConfig},
						{Key: "cleanup.policy", Value: &cleanup, Source: kmsg.ConfigSourceDefaultConfig},
					},
				},
			}, nil
		},
	}
	task := &topicConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "orders", data["topic"])
	assert.Equal(t, 2, data["count"])
	configs := data["configs"].([]ConfigEntry)
	require.Len(t, configs, 2)
	assert.Equal(t, "retention.ms", configs[0].Name)
}

func TestTopicConfigGetTaskExcludeDefaults(t *testing.T) {
	t.Parallel()
	retention := "604800000"
	cleanup := "delete"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeTopicConfigsFunc: func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name: "orders",
					Configs: []kadm.Config{
						{Key: "retention.ms", Value: &retention, Source: kmsg.ConfigSourceDynamicTopicConfig},
						{Key: "cleanup.policy", Value: &cleanup, Source: kmsg.ConfigSourceDefaultConfig},
					},
				},
			}, nil
		},
	}
	task := &topicConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "include_defaults": false})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	configs := data["configs"].([]ConfigEntry)
	assert.Equal(t, "retention.ms", configs[0].Name)
}

func TestTopicConfigGetTaskFilterKeys(t *testing.T) {
	t.Parallel()
	retention := "604800000"
	cleanup := "delete"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeTopicConfigsFunc: func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name: "orders",
					Configs: []kadm.Config{
						{Key: "retention.ms", Value: &retention, Source: kmsg.ConfigSourceDynamicTopicConfig},
						{Key: "cleanup.policy", Value: &cleanup, Source: kmsg.ConfigSourceDefaultConfig},
					},
				},
			}, nil
		},
	}
	task := &topicConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "keys": []any{"retention.ms"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	configs := data["configs"].([]ConfigEntry)
	assert.Equal(t, "retention.ms", configs[0].Name)
}
