package linux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// blockdeviceListTask implements the linux.blockdevice.list capability.
type blockdeviceListTask struct{ provider *Provider }

const blockdeviceListSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.blockdevice.list Parameters",
  "description": "Parameters for linux.blockdevice.list.",
  "properties": {},
  "required": []
}`

func (t *blockdeviceListTask) Name() string       { return "linux.blockdevice.list" }
func (t *blockdeviceListTask) JSONSchema() string { return blockdeviceListSchema }

func (t *blockdeviceListTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	devices, err := readBlockDevices()
	if err != nil {
		return common.TaskFailure(fmt.Errorf("read block devices: %w", err))
	}

	return common.SuccessResult(map[string]any{
		"devices": devices,
		"count":   len(devices),
	}), nil
}

func readBlockDevices() ([]map[string]any, error) {
	// Read /sys/block to get top-level block devices.
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}

	var devices []map[string]any
	for _, entry := range entries {
		name := entry.Name()
		// Skip non-disk entries (like loop devices unless you want them).
		// Include everything for completeness.
		devPath := filepath.Join("/sys/block", name)
		device := map[string]any{
			"name": name,
			"type": "disk",
		}

		// Read size in sectors (512 bytes each)
		sizeSectors, err := readSysBlockSize(devPath)
		if err == nil {
			device["size_bytes"] = sizeSectors * 512
		}

		// Check for partitions
		partEntries, err := os.ReadDir(devPath)
		if err == nil {
			var partitions []map[string]any
			for _, pe := range partEntries {
				if !pe.IsDir() {
					continue
				}
				pname := pe.Name()
				// Partitions start with the device name (e.g., sda1)
				if strings.HasPrefix(pname, name) {
					partPath := filepath.Join(devPath, pname)
					part := map[string]any{
						"name": pname,
						"type": "partition",
					}
					psize, err := readSysBlockSize(partPath)
					if err == nil {
						part["size_bytes"] = psize * 512
					}
					partitions = append(partitions, part)
				}
			}
			if len(partitions) > 0 {
				device["partitions"] = partitions
			}
		}

		// Read /proc/partitions for additional info
		if major, minor, blocks, err := readProcPartitions(name); err == nil {
			device["major"] = major
			device["minor"] = minor
			if sizeSectors == 0 {
				device["size_bytes"] = blocks * 1024
			}
		}

		devices = append(devices, device)
	}

	return devices, nil
}

func readSysBlockSize(path string) (uint64, error) {
	data, err := os.ReadFile(filepath.Join(path, "size"))
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(data))
	return strconv.ParseUint(s, 10, 64)
}

func readProcPartitions(name string) (int, int, uint64, error) {
	data, err := os.ReadFile("/proc/partitions")
	if err != nil {
		return 0, 0, 0, err
	}
	return parseProcPartitionsContent(string(data), name)
}

func parseProcPartitionsContent(content string, name string) (int, int, uint64, error) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i < 2 {
			continue // skip header lines
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		if fields[3] == name {
			major, _ := strconv.Atoi(fields[0])
			minor, _ := strconv.Atoi(fields[1])
			blocks, _ := strconv.ParseUint(fields[2], 10, 64)
			return major, minor, blocks, nil
		}
	}
	return 0, 0, 0, fmt.Errorf("device %q not found in /proc/partitions", name)
}

var _ task.Task = (*blockdeviceListTask)(nil)
