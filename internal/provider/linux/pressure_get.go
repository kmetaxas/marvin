package linux

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// pressureGetTask implements the linux.pressure.get capability.
type pressureGetTask struct{ provider *Provider }

const pressureGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.pressure.get Parameters",
  "description": "Parameters for linux.pressure.get."
}`

func (t *pressureGetTask) Name() string       { return "linux.pressure.get" }
func (t *pressureGetTask) JSONSchema() string { return pressureGetSchema }

func (t *pressureGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("pressure.get starting", "capability", t.Name())

	data, err := readPressure(t.provider.CurrentReader())
	if err != nil {
		slog.Info("pressure.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("pressure.get succeeded", "capability", t.Name())
	return common.SuccessResult(data), nil
}

var _ task.Task = (*pressureGetTask)(nil)

type PressureResource struct {
	Some *PressureData `json:"some,omitempty"`
	Full *PressureData `json:"full,omitempty"`
}

type PressureData struct {
	Avg10  float64 `json:"avg10"`
	Avg60  float64 `json:"avg60"`
	Avg300 float64 `json:"avg300"`
	Total  uint64  `json:"total"`
}

func readPressure(r *procfs.Reader) (map[string]any, error) {
	resources := []string{"cpu", "memory", "io"}
	result := make(map[string]any)

	for _, res := range resources {
		lines, err := r.ReadFileLines("proc", "pressure", res)
		if err != nil {
			continue
		}
		pr := parsePressureResource(lines)
		if pr != nil {
			result[res] = pr
		}
	}

	return result, nil
}

func parsePressureResource(lines []string) *PressureResource {
	pr := &PressureResource{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "some ") {
			pr.Some = parsePressureLine(line)
		} else if strings.HasPrefix(line, "full ") {
			pr.Full = parsePressureLine(line)
		}
	}
	if pr.Some == nil && pr.Full == nil {
		return nil
	}
	return pr
}

func parsePressureLine(line string) *PressureData {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return nil
	}
	pd := &PressureData{}
	for _, f := range fields[1:] {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		valStr := parts[1]
		switch key {
		case "avg10":
			pd.Avg10, _ = strconv.ParseFloat(valStr, 64)
		case "avg60":
			pd.Avg60, _ = strconv.ParseFloat(valStr, 64)
		case "avg300":
			pd.Avg300, _ = strconv.ParseFloat(valStr, 64)
		case "total":
			pd.Total, _ = strconv.ParseUint(valStr, 10, 64)
		}
	}
	return pd
}
