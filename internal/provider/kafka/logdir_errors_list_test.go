package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestLogdirErrorsListTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &logdirErrorsListTask{}
	assert.Equal(t, "kafka.logdir.errors.list", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestLogdirErrorsListTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &logdirErrorsListTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestLogdirErrorsListTaskSuccess(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}},
					}},
				},
				2: {
					"/var/lib/kafka/data-1": {Broker: 2, Dir: "/var/lib/kafka/data-1", Err: errors.New("KafkaStorageException: Disk error")},
				},
			}, nil
		},
	}
	task := &logdirErrorsListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	errs := data["errors"].([]map[string]any)
	require.Len(t, errs, 1)
	assert.Equal(t, int32(2), errs[0]["broker_id"])
	assert.Equal(t, "/var/lib/kafka/data-1", errs[0]["log_dir"])
	assert.Contains(t, errs[0]["error"], "Disk error")
}

func TestLogdirErrorsListTaskNoErrors(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Topics: kadm.DescribedLogDirTopics{
						"orders": {0: {Size: 100}},
					}},
				},
			}, nil
		},
	}
	task := &logdirErrorsListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 0, data["count"])
	errs := data["errors"].([]map[string]any)
	assert.Empty(t, errs)
}

func TestLogdirErrorsListTaskBrokerFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return kadm.DescribedAllLogDirs{
				1: {
					"/var/lib/kafka/data-1": {Broker: 1, Dir: "/var/lib/kafka/data-1", Err: errors.New("err1")},
				},
				2: {
					"/var/lib/kafka/data-1": {Broker: 2, Dir: "/var/lib/kafka/data-1", Err: errors.New("err2")},
				},
			}, nil
		},
	}
	task := &logdirErrorsListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_ids": []any{2}})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	errs := data["errors"].([]map[string]any)
	assert.Equal(t, int32(2), errs[0]["broker_id"])
}

func TestLogdirErrorsListTaskClientError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		describeAllLogDirsFunc: func(ctx context.Context) (kadm.DescribedAllLogDirs, error) {
			return nil, errors.New("describe failed")
		},
	}
	task := &logdirErrorsListTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "describe failed")
}
