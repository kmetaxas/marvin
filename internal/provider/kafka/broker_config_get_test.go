package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestBrokerConfigGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &brokerConfigGetTask{}
	assert.Equal(t, "kafka.broker.config.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestBrokerConfigGetTaskValidation(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}
	task := &brokerConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "missing required parameter: broker_id")
}

func TestBrokerConfigGetTaskSuccess(t *testing.T) {
	t.Parallel()
	secret := "s3cr3t"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeConfigsFunc: func(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name: "1",
					Configs: []kadm.Config{
						{Key: "log.retention.hours", Value: &secret, Source: kmsg.ConfigSourceDefaultConfig},
						{Key: "password", Value: nil, Sensitive: true, Source: kmsg.ConfigSourceDynamicBrokerConfig},
					},
				},
			}, nil
		},
	}
	task := &brokerConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_id": 1})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
	configs := data["configs"].([]ConfigEntry)
	require.Len(t, configs, 2)
	assert.Equal(t, "log.retention.hours", configs[0].Name)
	assert.Equal(t, "s3cr3t", configs[0].Value)
	assert.True(t, configs[0].Default)
	assert.Equal(t, "password", configs[1].Name)
	assert.Equal(t, "[REDACTED]", configs[1].Value)
	assert.True(t, configs[1].Sensitive)
}

func TestBrokerConfigGetTaskKeyFilter(t *testing.T) {
	t.Parallel()
	v1 := "168"
	v2 := "2"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeConfigsFunc: func(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name: "1",
					Configs: []kadm.Config{
						{Key: "log.retention.hours", Value: &v1, Source: kmsg.ConfigSourceDefaultConfig},
						{Key: "min.insync.replicas", Value: &v2, Source: kmsg.ConfigSourceDynamicBrokerConfig},
					},
				},
			}, nil
		},
	}
	task := &brokerConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_id": 1, "keys": []any{"min.insync.replicas"}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	configs := data["configs"].([]ConfigEntry)
	assert.Equal(t, "min.insync.replicas", configs[0].Name)
}

func TestBrokerConfigGetTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeConfigsFunc: func(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
			return nil, errors.New("config failed")
		},
	}
	task := &brokerConfigGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_id": 1})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "config failed")
}
