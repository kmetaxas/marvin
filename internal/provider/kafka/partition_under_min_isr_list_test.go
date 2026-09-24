package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestPartitionUnderMinISRListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionUnderMinISRListTask{}
	assert.Equal(t, "kafka.partition.under_min_isr.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionUnderMinISRListTaskFilters(t *testing.T) {
	t.Parallel()
	minISR := "2"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
						1: {Topic: "orders", Partition: 1, Leader: 2, Replicas: []int32{2, 3, 1}, ISR: []int32{2}},
					},
				},
			}, nil
		},
		describeTopicConfigsFunc: func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name:    "orders",
					Configs: []kadm.Config{{Key: "min.insync.replicas", Value: &minISR, Source: kmsg.ConfigSourceDynamicTopicConfig}},
				},
			}, nil
		},
	}
	task := &partitionUnderMinISRListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	partitions := data["under_min_isr_partitions"].([]PartitionInfo)
	require.Len(t, partitions, 1)
	assert.Equal(t, int32(1), partitions[0].Partition)
}

func TestPartitionUnderMinISRListTaskNone(t *testing.T) {
	t.Parallel()
	minISR := "2"
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		listTopicsFunc: func(ctx context.Context, topics ...string) (kadm.TopicDetails, error) {
			return kadm.TopicDetails{
				"orders": {
					Topic: "orders",
					Partitions: map[int32]kadm.PartitionDetail{
						0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2, 3}, ISR: []int32{1, 2, 3}},
					},
				},
			}, nil
		},
		describeTopicConfigsFunc: func(ctx context.Context, topics ...string) (kadm.ResourceConfigs, error) {
			return kadm.ResourceConfigs{
				{
					Name:    "orders",
					Configs: []kadm.Config{{Key: "min.insync.replicas", Value: &minISR, Source: kmsg.ConfigSourceDynamicTopicConfig}},
				},
			}, nil
		},
	}
	task := &partitionUnderMinISRListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
}
