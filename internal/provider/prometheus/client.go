package prometheus

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// prometheusClient is the narrow interface exposed to tasks. It wraps v1.API
// with guardrail-injected timeouts and limits.
type prometheusClient interface {
	Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error)
	QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error)
	QueryExemplars(ctx context.Context, query string, startTime, endTime time.Time) ([]v1.ExemplarQueryResult, error)
	Series(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error)
	LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error)
	LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error)
	Metadata(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error)
	Alerts(ctx context.Context) (v1.AlertsResult, error)
	Rules(ctx context.Context, matches []string) (v1.RulesResult, error)
	Targets(ctx context.Context) (v1.TargetsResult, error)
	TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error)
	Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error)
	Buildinfo(ctx context.Context) (v1.BuildinfoResult, error)
	TSDB(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error)
	Flags(ctx context.Context) (v1.FlagsResult, error)
	Config(ctx context.Context) (v1.ConfigResult, error)
	Ready(ctx context.Context) (bool, error)
	Healthy(ctx context.Context) (bool, error)
	Guardrails() GuardrailPolicy
}

// clientImpl wraps the official v1.API with guardrails and auth.
type clientImpl struct {
	api        v1.API
	guardrails GuardrailPolicy
	baseURL    string
	httpClient *http.Client
	initErr    error
}

// newPrometheusClient is a swappable constructor for test injection.
var newPrometheusClient = func(cfg config.PrometheusConfig) *clientImpl {
	c := &clientImpl{}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}

	rt, err := buildAuthRoundTripper(cfg.Auth, httpClient.Transport)
	if err != nil {
		c.initErr = fmt.Errorf("build prometheus auth transport: %w", err)
		return c
	}
	httpClient.Transport = rt

	apiClient, err := api.NewClient(api.Config{
		Address: cfg.URL,
		Client:  httpClient,
	})
	if err != nil {
		c.initErr = fmt.Errorf("create prometheus api client: %w", err)
		return c
	}

	c.api = v1.NewAPI(apiClient)
	c.baseURL = cfg.URL
	c.httpClient = httpClient
	c.guardrails = GuardrailPolicy{
		MaxQueryRange:      cfg.Guardrails.MaxQueryRange,
		MinStep:            cfg.Guardrails.MinStep,
		QueryTimeout:       cfg.Guardrails.QueryTimeout,
		MaxReturnedSeries:  cfg.Guardrails.MaxReturnedSeries,
		MaxReturnedSamples: cfg.Guardrails.MaxReturnedSamples,
		MaxMetadataResults: cfg.Guardrails.MaxMetadataResults,
		MaxResponseBytes:   cfg.Guardrails.MaxResponseBytes,
	}
	c.guardrails.ApplyDefaults()
	return c
}

func (c *clientImpl) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	opts = append(opts, v1.WithTimeout(c.guardrails.QueryTimeout))
	return c.api.Query(ctx, query, ts, opts...)
}

func (c *clientImpl) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	opts = append(opts, v1.WithTimeout(c.guardrails.QueryTimeout))
	return c.api.QueryRange(ctx, query, r, opts...)
}

func (c *clientImpl) QueryExemplars(ctx context.Context, query string, startTime, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.QueryExemplars(ctx, query, startTime, endTime)
}

func (c *clientImpl) Series(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) ([]model.LabelSet, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Series(ctx, matches, startTime, endTime, opts...)
}

func (c *clientImpl) LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelNames, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.LabelNames(ctx, matches, startTime, endTime, opts...)
}

func (c *clientImpl) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (model.LabelValues, v1.Warnings, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.LabelValues(ctx, label, matches, startTime, endTime, opts...)
}

func (c *clientImpl) Metadata(ctx context.Context, metric, limit string) (map[string][]v1.Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Metadata(ctx, metric, limit)
}

func (c *clientImpl) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Alerts(ctx)
}

func (c *clientImpl) Rules(ctx context.Context, matches []string) (v1.RulesResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Rules(ctx, matches)
}

func (c *clientImpl) Targets(ctx context.Context) (v1.TargetsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Targets(ctx)
}

func (c *clientImpl) TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) ([]v1.MetricMetadata, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.TargetsMetadata(ctx, matchTarget, metric, limit)
}

func (c *clientImpl) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Runtimeinfo(ctx)
}

func (c *clientImpl) Buildinfo(ctx context.Context) (v1.BuildinfoResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Buildinfo(ctx)
}

func (c *clientImpl) TSDB(ctx context.Context, opts ...v1.Option) (v1.TSDBResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.TSDB(ctx, opts...)
}

func (c *clientImpl) Flags(ctx context.Context) (v1.FlagsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Flags(ctx)
}

func (c *clientImpl) Config(ctx context.Context) (v1.ConfigResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()
	return c.api.Config(ctx)
}

// Ready reports whether Prometheus is ready to serve queries by probing the
// /-/ready endpoint. A 200 response means ready; any other status means not
// ready. Transport errors are returned to the caller.
func (c *clientImpl) Ready(ctx context.Context) (bool, error) {
	return c.probeEndpoint(ctx, "/-/ready")
}

// Healthy reports Prometheus basic health by probing the /-/healthy endpoint.
// A 200 response means healthy; any other status means not healthy. Transport
// errors are returned to the caller.
func (c *clientImpl) Healthy(ctx context.Context) (bool, error) {
	return c.probeEndpoint(ctx, "/-/healthy")
}

// probeEndpoint performs a GET against a path on the Prometheus base URL and
// reports whether it returned HTTP 200.
func (c *clientImpl) probeEndpoint(ctx context.Context, path string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.guardrails.QueryTimeout)
	defer cancel()

	u := strings.TrimRight(c.baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK, nil
}

// Guardrails returns the guardrail policy in effect for this client.
func (c *clientImpl) Guardrails() GuardrailPolicy {
	return c.guardrails
}
