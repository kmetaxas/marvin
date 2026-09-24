package linux

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// systemInfoTask returns basic host information.
type systemInfoTask struct{ provider *Provider }

const systemInfoSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "System Info Parameters",
  "description": "Parameters for retrieving basic host information."
}`

func (t *systemInfoTask) Name() string       { return "linux.system.info" }
func (t *systemInfoTask) JSONSchema() string { return systemInfoSchema }

func (t *systemInfoTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = time.Now()
	slog.Info("system.info starting", "capability", t.Name())

	info, err := readSystemInfo(t.provider.CurrentReader())
	if err != nil {
		slog.Info("system.info failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("system.info succeeded", "capability", t.Name())
	return common.SuccessResult(info), nil
}

var _ task.Task = (*systemInfoTask)(nil)

func readSystemInfo(r *procfs.Reader) (map[string]any, error) {
	info := make(map[string]any)

	// Hostname
	hostname, err := os.Hostname()
	if err == nil {
		info["hostname"] = hostname
	}

	// OS info from /etc/os-release
	osInfo := readOSRelease(r)
	for k, v := range osInfo {
		info[k] = v
	}

	// Kernel and architecture from uname
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err == nil {
		info["kernel"] = charsToString(uname.Release)
		info["architecture"] = charsToString(uname.Machine)
	}

	// Boot ID
	if bootID, err := r.ReadFileString("proc", "sys", "kernel", "random", "boot_id"); err == nil {
		info["boot_id"] = strings.TrimSpace(bootID)
	}

	// Virtualization hint
	virt := detectVirtualization(r)
	if virt != "" {
		info["virtualization"] = virt
	}

	return info, nil
}

func readOSRelease(r *procfs.Reader) map[string]any {
	result := make(map[string]any)
	data, err := r.ReadFileString("etc", "os-release")
	if err != nil {
		return result
	}

	lines := strings.Split(data, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"`)

		switch key {
		case "NAME":
			result["os_name"] = value
		case "VERSION":
			result["os_version"] = value
		case "VERSION_ID":
			result["os_version_id"] = value
		case "ID":
			result["os_id"] = value
		case "ID_LIKE":
			result["os_id_like"] = value
		case "PRETTY_NAME":
			result["os_pretty_name"] = value
		}
	}
	return result
}

func detectVirtualization(r *procfs.Reader) string {
	// Check DMI product name
	if name, err := r.ReadFileString("sys", "class", "dmi", "id", "product_name"); err == nil {
		name = strings.TrimSpace(name)
		lower := strings.ToLower(name)
		switch {
		case strings.Contains(lower, "vmware"):
			return "vmware"
		case strings.Contains(lower, "virtualbox"):
			return "virtualbox"
		case strings.Contains(lower, "kvm") || strings.Contains(lower, "qemu"):
			return "kvm"
		case strings.Contains(lower, "xen"):
			return "xen"
		case strings.Contains(lower, "hyper-v"), strings.Contains(lower, "hyperv"):
			return "hyper-v"
		}
	}

	// Check /proc/cpuinfo for hypervisor flag
	if data, err := r.ReadFileString("proc", "cpuinfo"); err == nil {
		if strings.Contains(data, "hypervisor") {
			return "unknown_hypervisor"
		}
	}

	// Check /proc/1/cgroup for container indicators
	if data, err := r.ReadFileString("proc", "1", "cgroup"); err == nil {
		if strings.Contains(data, "docker") {
			return "docker"
		}
		if strings.Contains(data, "lxc") {
			return "lxc"
		}
		if strings.Contains(data, "containerd") {
			return "containerd"
		}
		if strings.Contains(data, "kubepods") {
			return "kubernetes"
		}
	}

	return ""
}

func charsToString(ca [65]int8) string {
	var bs []byte
	for _, c := range ca {
		if c == 0 {
			break
		}
		bs = append(bs, byte(c))
	}
	return string(bs)
}
