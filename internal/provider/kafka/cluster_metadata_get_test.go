package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestClusterMetadataGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &clusterMetadataGetTask{}
	assert.Equal(t, "kafka.cluster.metadata.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestClusterMetadataGetTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &clusterMetadataGetTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestClusterMetadataGetTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{
				Cluster:    "cluster-123",
				Controller: 1,
				Brokers:    kadm.BrokerDetails{{NodeID: 1, Host: "kafka-1", Port: 9092}},
				Topics: kadm.TopicDetails{
					"orders": {
						Topic: "orders",
						Partitions: kadm.PartitionDetails{
							0: {Topic: "orders", Partition: 0, Leader: 1, Replicas: []int32{1, 2}, ISR: []int32{1, 2}},
						},
					},
				},
			}, nil
		},
	}
	task := &clusterMetadataGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, "cluster-123", data["cluster_id"])
	assert.Equal(t, 1, data["topic_count"])
	assert.Equal(t, 1, data["partition_count"])
	assert.Equal(t, false, data["truncated"])

	topics := data["topics"].([]map[string]any)
	assert.Equal(t, "orders", topics[0]["topic"])
	partitions := topics[0]["partitions"].([]PartitionInfo)
	require.Len(t, partitions, 1)
	assert.Equal(t, int32(0), partitions[0].Partition)
}

func TestClusterMetadataGetTaskInternalFiltered(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{
				Topics: kadm.TopicDetails{
					"__consumer_offsets": {
						Topic:      "__consumer_offsets",
						IsInternal: true,
						Partitions: kadm.PartitionDetails{
							0: {Topic: "__consumer_offsets", Partition: 0, Leader: 1},
						},
					},
					"orders": {
						Topic: "orders",
						Partitions: kadm.PartitionDetails{
							0: {Topic: "orders", Partition: 0, Leader: 1},
						},
					},
				},
			}, nil
		},
	}
	task := &clusterMetadataGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["topic_count"])
	topics := data["topics"].([]map[string]any)
	assert.Equal(t, "orders", topics[0]["topic"])
}

func TestClusterMetadataGetTaskIncludeInternal(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{
				Topics: kadm.TopicDetails{
					"__consumer_offsets": {
						Topic:      "__consumer_offsets",
						IsInternal: true,
						Partitions: kadm.PartitionDetails{
							0: {Topic: "__consumer_offsets", Partition: 0, Leader: 1},
						},
					},
				},
			}, nil
		},
	}
	task := &clusterMetadataGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"include_internal": true})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["topic_count"])
}

func TestClusterMetadataGetTaskMaxPartitions(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		metadataFunc: func(ctx context.Context, topics ...string) (kadm.Metadata, error) {
			return kadm.Metadata{
				Topics: kadm.TopicDetails{
					"orders": {
						Topic: "orders",
						Partitions: kadm.PartitionDetails{
							0: {Topic: "orders", Partition: 0, Leader: 1},
							1: {Topic: "orders", Partition: 1, Leader: 1},
							2: {Topic: "orders", Partition: 2, Leader: 1},
						},
					},
				},
			}, nil
		},
	}
	task := &clusterMetadataGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"max_partitions": 2})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["partition_count"])
	assert.Equal(t, true, data["truncated"])
}
