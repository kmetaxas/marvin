package prometheus

import (
	"fmt"
	"time"

	"github.com/prometheus/common/model"
)

// metricToMap converts a model.Metric into a plain map[string]string.
func metricToMap(ls model.Metric) map[string]string {
	m := make(map[string]string, len(ls))
	for k, v := range ls {
		m[string(k)] = string(v)
	}
	return m
}

// labelSetToMap converts a model.LabelSet into a plain map[string]string.
func labelSetToMap(ls model.LabelSet) map[string]string {
	m := make(map[string]string, len(ls))
	for k, v := range ls {
		m[string(k)] = string(v)
	}
	return m
}

// sampleToMap converts a model.Sample into the structured result shape used by
// query tasks: {"metric": {...}, "value": [timestamp, "value"]}.
func sampleToMap(s *model.Sample) map[string]any {
	return map[string]any{
		"metric": metricToMap(s.Metric),
		"value":  []any{float64(s.Timestamp) / 1000, s.Value.String()},
	}
}

// sampleStreamToMap converts a model.SampleStream into the structured result
// shape used by range query tasks: {"metric": {...}, "values": [[ts, "v"], ...]}.
func sampleStreamToMap(ss *model.SampleStream) map[string]any {
	values := make([]any, 0, len(ss.Values))
	for _, p := range ss.Values {
		values = append(values, []any{float64(p.Timestamp) / 1000, p.Value.String()})
	}
	return map[string]any{
		"metric": metricToMap(ss.Metric),
		"values": values,
	}
}

// truncateVector truncates a model.Vector to at most maxSeries entries and
// maxSamples total samples. It returns the truncated vector and whether any
// truncation occurred.
func truncateVector(vec model.Vector, maxSeries, maxSamples int) (model.Vector, bool) {
	truncated := false
	if len(vec) > maxSeries {
		vec = vec[:maxSeries]
		truncated = true
	}
	total := 0
	for i := range vec {
		total++
		if total > maxSamples {
			vec = vec[:i]
			truncated = true
			break
		}
	}
	return vec, truncated
}

// optionalStringMap extracts an optional map[string]string parameter from the
// params map. It accepts both map[string]string and map[string]any values.
func optionalStringMap(params map[string]any, key string) (map[string]string, error) {
	v, ok := params[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch m := v.(type) {
	case map[string]string:
		return m, nil
	case map[string]any:
		out := make(map[string]string, len(m))
		for k, val := range m {
			s, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %s must be a map of strings", key)
			}
			out[k] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("parameter %s must be a map of strings", key)
	}
}

// labelsMatch reports whether every key/value pair in want is present with an
// equal value in have.
func labelsMatch(have model.LabelSet, want map[string]string) bool {
	for k, v := range want {
		if string(have[model.LabelName(k)]) != v {
			return false
		}
	}
	return true
}

// formatTime renders a time.Time as RFC3339, or an empty string for the zero
// time.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// truncateMatrix truncates a model.Matrix to at most maxSeries streams and
// maxSamples total samples. It returns the truncated matrix and whether any
// truncation occurred.
func truncateMatrix(mat model.Matrix, maxSeries, maxSamples int) (model.Matrix, bool) {
	truncated := false
	if len(mat) > maxSeries {
		mat = mat[:maxSeries]
		truncated = true
	}
	total := 0
	for i, ss := range mat {
		total += len(ss.Values)
		if total > maxSamples {
			remaining := maxSamples - (total - len(ss.Values))
			if remaining <= 0 {
				mat = mat[:i]
			} else {
				mat[i].Values = ss.Values[:remaining]
			}
			truncated = true
			break
		}
	}
	return mat, truncated
}
