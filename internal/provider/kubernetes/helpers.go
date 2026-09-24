package kubernetes

import (
	"errors"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var errClientNotConfigured = errors.New("kubernetes client is not configured")

func formatAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	return fmt.Sprintf("%dd", days)
}

func flattenResourceList(rl corev1.ResourceList) map[string]string {
	result := make(map[string]string, len(rl))
	for k, v := range rl {
		result[string(k)] = v.String()
	}
	keys := make([]string, 0, len(result))
	for k := range result {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sorted := make(map[string]string, len(result))
	for _, k := range keys {
		sorted[k] = result[k]
	}
	return sorted
}

type conditionSummary struct {
	Type   string `json:"type,omitempty"`
	Status string `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}
