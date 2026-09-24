package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestBrokerConfigDiffTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &brokerConfigDiffTask{}
	assert.Equal(t, "kafka.broker.config.diff", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestBrokerConfigDiffTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listBrokersFunc: func(ctx context.Context) (kadm.BrokerDetails, error) {
			return kadm.BrokerDetails{
				{NodeID: 1, Host: "kafka-1", Port: 9092},
				{NodeID: 2, Host: "kafka-2", Port: 9092},
				{NodeID: 3, Host: "kafka-3", Port: 9092},
			}, nil
		},
		describeConfigsFunc: func(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
			v2 := "2"
			v1 := "1"
			return kadm.ResourceConfigs{
				{
					Name: "1",
					Configs: []kadm.Config{
						{Key: "min.insync.replicas", Value: &v2, Source: kmsg.ConfigSourceDynamicBrokerConfig},
					},
				},
				{
					Name: "2",
					Configs: []kadm.Config{
						{Key: "min.insync.replicas", Value: &v1, Source: kmsg.ConfigSourceDynamicBrokerConfig},
					},
				},
				{
					Name: "3",
					Configs: []kadm.Config{
						{Key: "min.insync.replicas", Value: &v2, Source: kmsg.ConfigSourceDynamicBrokerConfig},
					},
				},
			}, nil
		},
	}
	task := &brokerConfigDiffTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	diff := data["diff"].([]map[string]any)
	require.Len(t, diff, 1)
	assert.Equal(t, "min.insync.replicas", diff[0]["key"])
	outliers := diff[0]["outlier_brokers"].([]int32)
	assert.Equal(t, []int32{2}, outliers)
}

func TestBrokerConfigDiffTaskNoDiff(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listBrokersFunc: func(ctx context.Context) (kadm.BrokerDetails, error) {
			return kadm.BrokerDetails{
				{NodeID: 1, Host: "kafka-1", Port: 9092},
				{NodeID: 2, Host: "kafka-2", Port: 9092},
			}, nil
		},
		describeConfigsFunc: func(ctx context.Context, brokers ...int32) (kadm.ResourceConfigs, error) {
			v := "2"
			return kadm.ResourceConfigs{
				{
					Name:    "1",
					Configs: []kadm.Config{{Key: "min.insync.replicas", Value: &v, Source: kmsg.ConfigSourceDynamicBrokerConfig}},
				},
				{
					Name:    "2",
					Configs: []kadm.Config{{Key: "min.insync.replicas", Value: &v, Source: kmsg.ConfigSourceDynamicBrokerConfig}},
				},
			}, nil
		},
	}
	task := &brokerConfigDiffTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
}
