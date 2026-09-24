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

// filesystemMountsTask implements the linux.filesystem.mounts capability.
type filesystemMountsTask struct{ provider *Provider }

const filesystemMountsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.filesystem.mounts Parameters",
  "description": "Parameters for linux.filesystem.mounts.",
  "properties": {},
  "required": []
}`

func (t *filesystemMountsTask) Name() string       { return "linux.filesystem.mounts" }
func (t *filesystemMountsTask) JSONSchema() string { return filesystemMountsSchema }

func (t *filesystemMountsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	mounts, err := readMountinfo()
	if err != nil {
		return common.TaskFailure(fmt.Errorf("read mountinfo: %w", err))
	}

	return common.SuccessResult(map[string]any{
		"mounts": mounts,
		"count":  len(mounts),
	}), nil
}

// mountinfoEntry represents a parsed /proc/self/mountinfo line.
type mountinfoEntry struct {
	MountID        int      `json:"mount_id"`
	ParentID       int      `json:"parent_id"`
	DeviceMajor    int      `json:"device_major"`
	DeviceMinor    int      `json:"device_minor"`
	Root           string   `json:"root"`
	MountPoint     string   `json:"mount_point"`
	Options        []string `json:"options"`
	FilesystemType string   `json:"filesystem_type"`
	Source         string   `json:"source"`
	SuperOptions   []string `json:"super_options,omitempty"`
}

func readMountinfo() ([]map[string]any, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var results []map[string]any
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry, ok := parseMountinfoLine(line)
		if ok {
			results = append(results, entry)
		}
	}
	return results, nil
}

func parseMountinfoLine(line string) (map[string]any, bool) {
	// Format from kernel Documentation/filesystems/proc.txt:
	// 36 35 98:0 /tmp1 /tmp2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
	// mount_id parent_id major:minor root mountpoint options optional_fields... - fs_type source super_options
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return nil, false
	}

	mountID, err1 := strconv.Atoi(fields[0])
	parentID, err2 := strconv.Atoi(fields[1])
	if err1 != nil || err2 != nil {
		return nil, false
	}

	devParts := strings.Split(fields[2], ":")
	if len(devParts) != 2 {
		return nil, false
	}
	major, err3 := strconv.Atoi(devParts[0])
	minor, err4 := strconv.Atoi(devParts[1])
	if err3 != nil || err4 != nil {
		return nil, false
	}

	root := fields[3]
	mountPoint := fields[4]
	options := strings.Split(fields[5], ",")

	// Find the separator "-"
	sepIdx := -1
	for i := 6; i < len(fields); i++ {
		if fields[i] == "-" {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 || len(fields) < sepIdx+3 {
		return nil, false
	}

	fsType := fields[sepIdx+1]
	source := fields[sepIdx+2]
	var superOptions []string
	if len(fields) > sepIdx+3 {
		superOptions = strings.Split(fields[sepIdx+3], ",")
	}

	return map[string]any{
		"mount_id":        mountID,
		"parent_id":       parentID,
		"device_major":    major,
		"device_minor":    minor,
		"root":            root,
		"mount_point":     mountPoint,
		"options":         options,
		"filesystem_type": fsType,
		"source":          source,
		"super_options":   superOptions,
	}, true
}

var _ task.Task = (*filesystemMountsTask)(nil)
