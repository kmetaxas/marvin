package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestClusterFeaturesGetTaskNameAndSchema(t *testing.T) {
	t.Parallel()
	task := &clusterFeaturesGetTask{}
	assert.Equal(t, "kafka.cluster.features.get", task.Name())
	assert.NotEmpty(t, task.JSONSchema())
}

func TestClusterFeaturesGetTaskNilClient(t *testing.T) {
	t.Parallel()
	task := &clusterFeaturesGetTask{}
	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "not configured")
}

func TestClusterFeaturesGetTaskFinalized(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		requestFunc: func(ctx context.Context, req kmsg.Request) (kmsg.Response, error) {
			return &kmsg.ApiVersionsResponse{
				FinalizedFeaturesEpoch: 1,
				FinalizedFeatures: []kmsg.ApiVersionsResponseFinalizedFeature{
					{Name: "metadata.version", MinVersionLevel: 1, MaxVersionLevel: 7},
				},
			}, nil
		},
	}
	task := &clusterFeaturesGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	assert.Equal(t, 1, data["count"])
	features := data["features"].([]map[string]any)
	assert.Equal(t, "metadata.version", features[0]["name"])
	assert.Equal(t, int16(7), features[0]["finalized_version_level"])
}

func TestClusterFeaturesGetTaskSupportedFallback(t *testing.T) {
	t.Parallel()
	client := &fakeKafkaClient{
		guardrails: DefaultGuardrailPolicy(),
		requestFunc: func(ctx context.Context, req kmsg.Request) (kmsg.Response, error) {
			return &kmsg.ApiVersionsResponse{
				FinalizedFeaturesEpoch: -1,
				SupportedFeatures: []kmsg.ApiVersionsResponseSupportedFeature{
					{Name: "metadata.version", MinVersion: 1, MaxVersion: 7},
				},
			}, nil
		},
	}
	task := &clusterFeaturesGetTask{client: client}

	res, err := task.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data := res.Data.(map[string]any)
	features := data["features"].([]map[string]any)
	assert.Equal(t, "metadata.version", features[0]["name"])
	assert.Equal(t, int16(7), features[0]["max_version_level"])
}
