package kafka

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
)

func TestClusterApiVersionsGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &clusterApiVersionsGetTask{}
	assert.Equal(t, "kafka.cluster.api_versions.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestClusterApiVersionsGetTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &clusterApiVersionsGetTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestClusterApiVersionsGetTaskBrokerFilter(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		apiVersionsFunc: func(ctx context.Context) (kadm.BrokersApiVersions, error) {
			return kadm.BrokersApiVersions{
				1: {NodeID: 1},
				2: {NodeID: 2},
			}, nil
		},
	}
	task := &clusterApiVersionsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{"broker_id": 2})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	apiVersions := data["api_versions"].([]map[string]any)
	assert.Equal(t, int32(2), apiVersions[0]["broker_id"])
}

func TestClusterApiVersionsGetTaskAllBrokers(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		apiVersionsFunc: func(ctx context.Context) (kadm.BrokersApiVersions, error) {
			return kadm.BrokersApiVersions{
				1: {NodeID: 1},
				2: {NodeID: 2},
			}, nil
		},
	}
	task := &clusterApiVersionsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 2, data["count"])
}

func TestClusterApiVersionsGetTaskError(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		apiVersionsFunc: func(ctx context.Context) (kadm.BrokersApiVersions, error) {
			return nil, errors.New("api versions failed")
		},
	}
	task := &clusterApiVersionsGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "api versions failed")
}
