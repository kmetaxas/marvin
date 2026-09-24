package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

var _ task.Task = (*podLogsTask)(nil)

type podLogsTask struct {
	provider    *Provider
	maxLogLines int
}

func (t *podLogsTask) Name() string { return "kubernetes.pod.logs" }

func (t *podLogsTask) JSONSchema() string { return podLogsSchema }

func (t *podLogsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	namespace, err := common.RequireString(params, "namespace")
	if err != nil {
		return common.TaskFailure(err)
	}
	name, err := common.RequireString(params, "name")
	if err != nil {
		return common.TaskFailure(err)
	}
	container, err := common.OptionalString(params, "container", "")
	if err != nil {
		return common.TaskFailure(err)
	}
	tailLines, err := common.OptionalInt(params, "tail_lines", 100)
	if err != nil {
		return common.TaskFailure(err)
	}
	previous, err := common.OptionalBool(params, "previous", false)
	if err != nil {
		return common.TaskFailure(err)
	}
	search, err := common.OptionalString(params, "search", "")
	if err != nil {
		return common.TaskFailure(err)
	}

	client := t.provider.CurrentClient()
	if client == nil || client.clientset == nil {
		return common.TaskFailure(errClientNotConfigured)
	}

	maxLimit := t.maxLogLines
	if maxLimit <= 0 {
		maxLimit = 100
	}
	if tailLines > maxLimit {
		tailLines = maxLimit
	}

	fetchLines := int64(tailLines)
	if search != "" {
		fetchLines = 2000
		if maxLimit < 2000 {
			fetchLines = int64(maxLimit)
		}
	}

	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &fetchLines,
		Previous:  previous,
	}

	raw, err := client.clientset.CoreV1().Pods(namespace).GetLogs(name, opts).DoRaw(ctx)
	if err != nil {
		return common.TaskFailure(err)
	}

	lines := splitLines(raw)
	output := extractLogContext(lines, search, tailLines)

	return common.SuccessResult(map[string]any{
		"logs":       output,
		"line_count": len(output),
		"searched":   search != "",
	}), nil
}

func splitLines(data []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func extractLogContext(lines []string, keyword string, maxContextLines int) []string {
	if keyword == "" {
		if len(lines) <= maxContextLines {
			return lines
		}
		return lines[len(lines)-maxContextLines:]
	}

	lowerKeyword := strings.ToLower(keyword)
	var matches []int
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerKeyword) {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return []string{}
	}

	halfContext := maxContextLines / 2
	if halfContext < 1 {
		halfContext = 1
	}

	type window struct{ start, end int }
	windows := make([]window, 0, len(matches))
	for _, idx := range matches {
		start := idx - halfContext
		if start < 0 {
			start = 0
		}
		end := idx + halfContext + 1
		if end > len(lines) {
			end = len(lines)
		}
		windows = append(windows, window{start, end})
	}

	merged := []window{windows[0]}
	for i := 1; i < len(windows); i++ {
		last := &merged[len(merged)-1]
		if windows[i].start <= last.end {
			if windows[i].end > last.end {
				last.end = windows[i].end
			}
		} else {
			merged = append(merged, windows[i])
		}
	}

	var result []string
	for _, w := range merged {
		for i := w.start; i < w.end && len(result) < maxContextLines; i++ {
			result = append(result, lines[i])
		}
		if len(result) >= maxContextLines {
			break
		}
	}

	return result
}

const podLogsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "Pod Logs Parameters",
  "description": "Parameters for retrieving pod container logs. Supports tailing the last N lines, fetching previous container logs, and optionally searching for a keyword within a larger buffer. When a search keyword is provided, the capability reads up to 2000 lines (or the configured maximum), finds occurrences of the keyword, and returns the surrounding lines up to tail_lines. Returns empty logs if the keyword is not found.",
  "properties": {
    "namespace": {
      "type": "string",
      "description": "Namespace containing the pod."
    },
    "name": {
      "type": "string",
      "description": "Name of the pod."
    },
    "container": {
      "type": "string",
      "description": "Specific container name within the pod. Required when the pod has multiple containers."
    },
    "tail_lines": {
      "type": "integer",
      "description": "Maximum number of log lines to return. Hard limited by the provider configuration (default 100).",
      "minimum": 1,
      "maximum": 10000,
      "default": 100
    },
    "previous": {
      "type": "boolean",
      "description": "Return logs from the previous container instance (useful after a restart).",
      "default": false
    },
    "search": {
      "type": "string",
      "description": "Optional keyword to search for. When provided, the capability reads up to 2000 lines (or the configured max), finds occurrences of the keyword, and returns the surrounding lines up to tail_lines. Returns empty logs if the keyword is not found."
    }
  },
  "required": ["namespace", "name"]
}`
