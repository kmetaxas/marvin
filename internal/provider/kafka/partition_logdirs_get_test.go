package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestPartitionLogdirsGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &partitionLogdirsGetTask{}
	assert.Equal(t, "kafka.partition.logdirs.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestPartitionLogdirsGetTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {
						Broker: 1,
						Dir:    "/var/lib/kafka/data-1",
						Topics: kadm.DescribedLogDirTopics{
							"orders": {
								0: {Broker: 1, Dir: "/var/lib/kafka/data-1", Topic: "orders", Partition: 0, Size: 1073741824},
							},
						},
					},
				},
			}, nil
		},
	}
	task := &partitionLogdirsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	logdirs := data["logdirs"].([]map[string]any)
	require.Len(t, logdirs, 1)
	assert.Equal(t, int32(1), logdirs[0]["broker_id"])
	assert.Equal(t, "orders", logdirs[0]["topic"])
	assert.Equal(t, int32(0), logdirs[0]["partition"])
	assert.Equal(t, int64(1073741824), logdirs[0]["size_bytes"])
}

func TestPartitionLogdirsGetTaskTopicFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {
						Broker: 1,
						Dir:    "/var/lib/kafka/data-1",
						Topics: kadm.DescribedLogDirTopics{
							"orders":   {0: {Broker: 1, Dir: "/var/lib/kafka/data-1", Topic: "orders", Partition: 0, Size: 100}},
							"payments": {0: {Broker: 1, Dir: "/var/lib/kafka/data-1", Topic: "payments", Partition: 0, Size: 200}},
						},
					},
				},
			}, nil
		},
	}
	task := &partitionLogdirsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"topic": "orders"})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	logdirs := data["logdirs"].([]map[string]any)
	assert.Equal(t, "orders", logdirs[0]["topic"])
}
