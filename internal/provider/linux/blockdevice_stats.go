package linux

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// blockdeviceStatsTask implements the linux.blockdevice.stats capability.
type blockdeviceStatsTask struct{ provider *Provider }

const blockdeviceStatsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.blockdevice.stats Parameters",
  "description": "Parameters for linux.blockdevice.stats.",
  "properties": {},
  "required": []
}`

func (t *blockdeviceStatsTask) Name() string       { return "linux.blockdevice.stats" }
func (t *blockdeviceStatsTask) JSONSchema() string { return blockdeviceStatsSchema }

func (t *blockdeviceStatsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	stats, err := readDiskstats()
	if err != nil {
		return common.TaskFailure(fmt.Errorf("read diskstats: %w", err))
	}

	return common.SuccessResult(map[string]any{
		"devices": stats,
		"count":   len(stats),
	}), nil
}

// DiskstatsEntry represents a parsed /proc/diskstats line.
// Fields per kernel documentation (Documentation/iostats.txt):
//
//	1  major number
//	2  minor number
//	3  device name
//	4  reads completed
//	5  reads merged
//	6  sectors read
//	7  ms spent reading
//	8  writes completed
//	9  writes merged
//
// 10  sectors written
// 11  ms spent writing
// 12  IOs currently in progress
// 13  ms spent doing IOs
// 14  weighted ms spent doing IOs
func readDiskstats() ([]map[string]any, error) {
	data, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	return parseDiskstatsContent(string(data)), nil
}

func parseDiskstatsContent(content string) []map[string]any {
	lines := strings.Split(content, "\n")
	var results []map[string]any
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 14 {
			continue
		}

		major, _ := strconv.Atoi(fields[0])
		minor, _ := strconv.Atoi(fields[1])
		name := fields[2]

		readCompleted, _ := strconv.ParseUint(fields[3], 10, 64)
		readMerged, _ := strconv.ParseUint(fields[4], 10, 64)
		sectorsRead, _ := strconv.ParseUint(fields[5], 10, 64)
		msReading, _ := strconv.ParseUint(fields[6], 10, 64)
		writeCompleted, _ := strconv.ParseUint(fields[7], 10, 64)
		writeMerged, _ := strconv.ParseUint(fields[8], 10, 64)
		sectorsWritten, _ := strconv.ParseUint(fields[9], 10, 64)
		msWriting, _ := strconv.ParseUint(fields[10], 10, 64)
		iosInProgress, _ := strconv.ParseUint(fields[11], 10, 64)
		msDoingIO, _ := strconv.ParseUint(fields[12], 10, 64)
		weightedMsDoingIO, _ := strconv.ParseUint(fields[13], 10, 64)

		results = append(results, map[string]any{
			"name":                 name,
			"major":                major,
			"minor":                minor,
			"reads_completed":      readCompleted,
			"reads_merged":         readMerged,
			"sectors_read":         sectorsRead,
			"ms_reading":           msReading,
			"writes_completed":     writeCompleted,
			"writes_merged":        writeMerged,
			"sectors_written":      sectorsWritten,
			"ms_writing":           msWriting,
			"ios_in_progress":      iosInProgress,
			"ms_doing_io":          msDoingIO,
			"weighted_ms_doing_io": weightedMsDoingIO,
		})
	}

	return results
}

var _ task.Task = (*blockdeviceStatsTask)(nil)
