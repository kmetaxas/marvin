package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestPartitionOffsetsCompareTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionOffsetsCompareTask{}
	assert.Equal(t, "kafka.partition.offsets.compare", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionOffsetsCompareTaskMissingParams(t *testing.T) {
	t.Parallel()
	task := &partitionOffsetsCompareTask{client: &fakeKafkaClient{guardrails: DefaultGuardrailPolicy()}}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
}

func TestPartitionOffsetsCompareTaskDetectsLag(t *testing.T) {
	t.Parallel()
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
		listEndOffsetsFunc: func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
			return kadm.ListedOffsets{
				"orders": {
					0: {Topic: "orders", Partition: 0, Offset: 15234567},
				},
			}, nil
		},
		requestShardedFunc: func(ctx context.Context, req kmsg.Request) []kgo.ResponseShard {
			loReq := req.(*kmsg.ListOffsetsRequest)
			offset := int64(15234567)
			switch loReq.ReplicaID {
			case 1:
				offset = 15234567
			case 2:
				offset = 15234566
			case 3:
				offset = 15230000
			}
			resp := kmsg.NewPtrListOffsetsResponse()
			resp.Topics = []kmsg.ListOffsetsResponseTopic{
				{
					Topic: "orders",
					Partitions: []kmsg.ListOffsetsResponseTopicPartition{
						{Partition: 0, Offset: offset},
					},
				},
			}
			return []kgo.ResponseShard{
				{Meta: kgo.BrokerMetadata{NodeID: loReq.ReplicaID}, Resp: resp},
			}
		},
	}
	task := &partitionOffsetsCompareTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "partition": 0})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.True(t, data["replica_lag_detected"].(bool))
	assert.Equal(t, int64(4567), data["max_lag"])
	replicas := data["replicas"].([]map[string]any)
	require.Len(t, replicas, 3)
	assert.Equal(t, int32(1), replicas[0]["broker_id"])
	assert.True(t, replicas[0]["leader"].(bool))
	assert.Equal(t, int64(1), replicas[1]["lag"])
	assert.Equal(t, int64(4567), replicas[2]["lag"])
}

func TestPartitionOffsetsCompareTaskNoLag(t *testing.T) {
	t.Parallel()
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
		listEndOffsetsFunc: func(ctx context.Context, topics ...string) (kadm.ListedOffsets, error) {
			return kadm.ListedOffsets{
				"orders": {
					0: {Topic: "orders", Partition: 0, Offset: 15234567},
				},
			}, nil
		},
		requestShardedFunc: func(ctx context.Context, req kmsg.Request) []kgo.ResponseShard {
			loReq := req.(*kmsg.ListOffsetsRequest)
			resp := kmsg.NewPtrListOffsetsResponse()
			resp.Topics = []kmsg.ListOffsetsResponseTopic{
				{
					Topic: "orders",
					Partitions: []kmsg.ListOffsetsResponseTopicPartition{
						{Partition: 0, Offset: 15234567},
					},
				},
			}
			return []kgo.ResponseShard{
				{Meta: kgo.BrokerMetadata{NodeID: loReq.ReplicaID}, Resp: resp},
			}
		},
	}
	task := &partitionOffsetsCompareTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders", "partition": 0})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.False(t, data["replica_lag_detected"].(bool))
	assert.Equal(t, int64(0), data["max_lag"])
}
