package prometheus

import (
	"context"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// fakePrometheusClient implements prometheusClient for tests. Each method is
// backed by a func field so individual tests can inject canned behavior.
type fakePrometheusClient struct {
	queryFunc           func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error)
	queryRangeFunc      func(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error)
	queryExemplarsFunc  func(ctx context.Context, query string, startTime, endTime time.Time) ([]v1.ExemplarQueryResult, error)
	seriesFunc          func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error)
	labelNamesFunc      func(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error)
	labelValuesFunc     func(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error)
	metadataFunc        func(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error)
	alertsFunc          func(ctx context.Context) (v1.AlertsResult, error)
	rulesFunc           func(ctx context.Context, matches []string) (v1.RulesResult, error)
	targetsFunc         func(ctx context.Context) (v1.TargetsResult, error)
	targetsMetadataFunc func(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error)
	runtimeinfoFunc     func(ctx context.Context) (v1.RuntimeinfoResult, error)
	buildinfoFunc       func(ctx context.Context) (v1.BuildinfoResult, error)
	tsdbFunc            func(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error)
	flagsFunc           func(ctx context.Context) (v1.FlagsResult, error)
	configFunc          func(ctx context.Context) (v1.ConfigResult, error)
	readyFunc           func(ctx context.Context) (bool, error)
	healthyFunc         func(ctx context.Context) (bool, error)
	guardrails          GuardrailPolicy
}

func (f *fakePrometheusClient) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	if f.queryFunc == nil {
		return nil, nil, nil
	}
	return f.queryFunc(ctx, query, ts, opts...)
}

func (f *fakePrometheusClient) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	if f.queryRangeFunc == nil {
		return nil, nil, nil
	}
	return f.queryRangeFunc(ctx, query, r, opts...)
}

func (f *fakePrometheusClient) QueryExemplars(ctx context.Context, query string, startTime, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
	if f.queryExemplarsFunc == nil {
		return nil, nil
	}
	return f.queryExemplarsFunc(ctx, query, startTime, endTime)
}

func (f *fakePrometheusClient) Series(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
	if f.seriesFunc == nil {
		return nil, nil, nil
	}
	return f.seriesFunc(ctx, matches, startTime, endTime, opts...)
}

func (f *fakePrometheusClient) LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error) {
	if f.labelNamesFunc == nil {
		return nil, nil, nil
	}
	return f.labelNamesFunc(ctx, matches, startTime, endTime, opts...)
}

func (f *fakePrometheusClient) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
	if f.labelValuesFunc == nil {
		return nil, nil, nil
	}
	return f.labelValuesFunc(ctx, label, matches, startTime, endTime, opts...)
}

func (f *fakePrometheusClient) Metadata(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
	if f.metadataFunc == nil {
		return nil, nil
	}
	return f.metadataFunc(ctx, metric, limit)
}

func (f *fakePrometheusClient) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	if f.alertsFunc == nil {
		return v1.AlertsResult{}, nil
	}
	return f.alertsFunc(ctx)
}

func (f *fakePrometheusClient) Rules(ctx context.Context, matches []string) (v1.RulesResult, error) {
	if f.rulesFunc == nil {
		return v1.RulesResult{}, nil
	}
	return f.rulesFunc(ctx, matches)
}

func (f *fakePrometheusClient) Targets(ctx context.Context) (v1.TargetsResult, error) {
	if f.targetsFunc == nil {
		return v1.TargetsResult{}, nil
	}
	return f.targetsFunc(ctx)
}

func (f *fakePrometheusClient) TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
	if f.targetsMetadataFunc == nil {
		return nil, nil
	}
	return f.targetsMetadataFunc(ctx, matchTarget, metric, limit)
}

func (f *fakePrometheusClient) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	if f.runtimeinfoFunc == nil {
		return v1.RuntimeinfoResult{}, nil
	}
	return f.runtimeinfoFunc(ctx)
}

func (f *fakePrometheusClient) Buildinfo(ctx context.Context) (v1.BuildinfoResult, error) {
	if f.buildinfoFunc == nil {
		return v1.BuildinfoResult{}, nil
	}
	return f.buildinfoFunc(ctx)
}

func (f *fakePrometheusClient) TSDB(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
	if f.tsdbFunc == nil {
		return v1.TSDBResult{}, nil
	}
	return f.tsdbFunc(ctx, opts...)
}

func (f *fakePrometheusClient) Flags(ctx context.Context) (v1.FlagsResult, error) {
	if f.flagsFunc == nil {
		return v1.FlagsResult{}, nil
	}
	return f.flagsFunc(ctx)
}

func (f *fakePrometheusClient) Config(ctx context.Context) (v1.ConfigResult, error) {
	if f.configFunc == nil {
		return v1.ConfigResult{}, nil
	}
	return f.configFunc(ctx)
}

func (f *fakePrometheusClient) Ready(ctx context.Context) (bool, error) {
	if f.readyFunc == nil {
		return false, nil
	}
	return f.readyFunc(ctx)
}

func (f *fakePrometheusClient) Healthy(ctx context.Context) (bool, error) {
	if f.healthyFunc == nil {
		return false, nil
	}
	return f.healthyFunc(ctx)
}

func (f *fakePrometheusClient) Guardrails() GuardrailPolicy {
	return f.guardrails
}
