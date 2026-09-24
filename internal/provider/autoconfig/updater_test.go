package autoconfig

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testClient struct {
	id     string
	closed bool
}

type testConfig struct {
	Value          string
	RuntimeVersion string
}

func TestApplyNoChange(t *testing.T) {
	t.Parallel()
	mu := &sync.RWMutex{}
	client := &testClient{id: "initial"}
	state := NewState(mu, testConfig{Value: "same"}, client)

	changed, recreated, err := Apply(state, map[string]any{"value": "same"}, Options[testConfig, *testClient]{
		Parse: parseTestConfig,
		Build: func(cfg testConfig) (*testClient, error) {
			t.Fatal("build should not run")
			return nil, nil
		},
	})

	require.NoError(t, err)
	assert.False(t, changed)
	assert.False(t, recreated)
	assert.Same(t, client, state.Client())
}

func TestApplyChangeBuildClose(t *testing.T) {
	t.Parallel()
	mu := &sync.RWMutex{}
	oldClient := &testClient{id: "old"}
	state := NewState(mu, testConfig{Value: "old"}, oldClient)
	var changedHookCalled bool

	changed, recreated, err := Apply(state, map[string]any{"value": "new"}, Options[testConfig, *testClient]{
		Parse: parseTestConfig,
		Build: func(cfg testConfig) (*testClient, error) {
			return &testClient{id: cfg.Value}, nil
		},
		Close: func(client *testClient) { client.closed = true },
		OnChanged: func(oldCfg, newCfg testConfig, hookRecreated bool) {
			changedHookCalled = true
			assert.Equal(t, "old", oldCfg.Value)
			assert.Equal(t, "new", newCfg.Value)
			assert.True(t, hookRecreated)
		},
	})

	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, recreated)
	assert.True(t, changedHookCalled)
	assert.True(t, oldClient.closed)
	assert.Equal(t, testConfig{Value: "new"}, state.Config())
	assert.Equal(t, "new", state.Client().id)
}

func TestApplyBuildError(t *testing.T) {
	t.Parallel()
	mu := &sync.RWMutex{}
	oldClient := &testClient{id: "old"}
	state := NewState(mu, testConfig{Value: "old"}, oldClient)
	buildErr := errors.New("build failed")
	var hookErr error

	changed, recreated, err := Apply(state, map[string]any{"value": "new"}, Options[testConfig, *testClient]{
		Parse:   parseTestConfig,
		Build:   func(testConfig) (*testClient, error) { return nil, buildErr },
		OnError: func(err error) { hookErr = err },
	})

	require.ErrorIs(t, err, buildErr)
	assert.Same(t, buildErr, hookErr)
	assert.False(t, changed)
	assert.False(t, recreated)
	assert.Equal(t, testConfig{Value: "old"}, state.Config())
	assert.Same(t, oldClient, state.Client())
}

func TestApplySelectiveRecreate(t *testing.T) {
	t.Parallel()
	mu := &sync.RWMutex{}
	oldClient := &testClient{id: "old"}
	state := NewState(mu, testConfig{Value: "old", RuntimeVersion: "v1"}, oldClient)

	changed, recreated, err := Apply(state, map[string]any{"value": "new", "runtime_version": "v1"}, Options[testConfig, *testClient]{
		Parse: parseTestConfig,
		Recreate: func(oldCfg, newCfg testConfig) bool {
			return oldCfg.RuntimeVersion != newCfg.RuntimeVersion
		},
		Build: func(cfg testConfig) (*testClient, error) {
			t.Fatal("build should not run")
			return nil, nil
		},
		Close: func(client *testClient) { client.closed = true },
	})

	require.NoError(t, err)
	assert.True(t, changed)
	assert.False(t, recreated)
	assert.False(t, oldClient.closed)
	assert.Same(t, oldClient, state.Client())
	assert.Equal(t, testConfig{Value: "new", RuntimeVersion: "v1"}, state.Config())
}

func TestApplyLockTimeEqualityDiscard(t *testing.T) {
	t.Parallel()
	mu := &sync.RWMutex{}
	state := NewState(mu, testConfig{Value: "old"}, &testClient{id: "old"})
	speculative := &testClient{id: "new"}
	parseEntered := make(chan struct{})
	allowParse := make(chan struct{})

	done := make(chan struct {
		changed   bool
		recreated bool
		err       error
	})
	go func() {
		changed, recreated, err := Apply(state, map[string]any{"value": "new"}, Options[testConfig, *testClient]{
			Parse: func(base testConfig, raw map[string]any) (testConfig, error) {
				close(parseEntered)
				<-allowParse
				return parseTestConfig(base, raw)
			},
			Build: func(testConfig) (*testClient, error) { return speculative, nil },
			Close: func(client *testClient) { client.closed = true },
		})
		done <- struct {
			changed   bool
			recreated bool
			err       error
		}{changed: changed, recreated: recreated, err: err}
	}()

	<-parseEntered
	mu.Lock()
	state.config = testConfig{Value: "new"}
	state.client = &testClient{id: "winner"}
	mu.Unlock()
	close(allowParse)

	result := <-done
	require.NoError(t, result.err)
	assert.False(t, result.changed)
	assert.False(t, result.recreated)
	assert.True(t, speculative.closed)
	assert.Equal(t, "winner", state.Client().id)
}

func parseTestConfig(base testConfig, raw map[string]any) (testConfig, error) {
	out := base
	if v, ok := StringOK(raw, "value"); ok {
		out.Value = v
	}
	if v, ok := StringOK(raw, "runtime_version"); ok {
		out.RuntimeVersion = v
	}
	return out, nil
}
