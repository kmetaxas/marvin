package linux

import (
	"runtime"
	"sync"

	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

// Provider implements provider.Provider for Linux system introspection.
type Provider struct {
	mu     sync.RWMutex
	config config.LinuxConfig
	reader *procfs.Reader
	provider.BaseProvider
}

// NewProvider creates a new Linux provider with the given configuration.
func NewProvider(cfg config.LinuxConfig) *Provider {
	p := &Provider{
		config: applyLinuxDefaults(cfg),
		reader: procfs.DefaultReader,
	}

	// OS gate: only register on Linux.
	if runtime.GOOS != "linux" {
		p.BaseProvider = provider.BaseProvider{
			ProviderName:         "linux",
			ProviderCapabilities: []capability.Capability{},
			Tasks:                map[string]task.Task{},
		}
		return p
	}

	tasks := buildTasks(p)
	capabilities := make([]capability.Capability, 0, len(tasks))
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		capabilities = append(capabilities, capability.Capability{
			Name:                 t.Name(),
			Version:              "v1",
			Description:          capabilityDescription(t.Name()),
			Provider:             "linux",
			ParametersJSONSchema: t.JSONSchema(),
		})
		taskMap[t.Name()] = t
	}

	p.BaseProvider = provider.BaseProvider{
		ProviderName:         "linux",
		ProviderCapabilities: capabilities,
		Tasks:                taskMap,
	}
	return p
}

func (p *Provider) IsConfigured() bool { return runtime.GOOS == "linux" }

func (p *Provider) UpdateConfig(capabilityName string, cfg map[string]any) {
	// No per-capability runtime config for Linux provider.
}

// CurrentConfig returns the provider configuration safely.
func (p *Provider) CurrentConfig() config.LinuxConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// CurrentReader returns the procfs reader safely.
func (p *Provider) CurrentReader() *procfs.Reader {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.reader
}

func applyLinuxDefaults(cfg config.LinuxConfig) config.LinuxConfig {
	if cfg.AllowedReadPaths == nil {
		cfg.AllowedReadPaths = DefaultAllowedReadPaths()
	}
	if cfg.MaxReadBytes == 0 {
		cfg.MaxReadBytes = 65536
	}
	if cfg.MaxDirectoryDepth == 0 {
		cfg.MaxDirectoryDepth = 3
	}
	if cfg.MaxDirectoryEntries == 0 {
		cfg.MaxDirectoryEntries = 1000
	}
	if cfg.RedactSecrets == nil {
		redact := true
		cfg.RedactSecrets = &redact
	}
	if cfg.SecretPatterns == nil {
		cfg.SecretPatterns = DefaultSecretPatterns()
	}
	if cfg.JournalDirectory == "" {
		cfg.JournalDirectory = "/var/log/journal"
	}
	if cfg.SystemdBusAddress == "" {
		cfg.SystemdBusAddress = "unix:path=/run/dbus/system_bus_socket"
	}
	return cfg
}

func buildTasks(p *Provider) []task.Task {
	return []task.Task{
		&systemInfoTask{provider: p},
		&systemUptimeTask{provider: p},
		&systemTimeTask{provider: p},
		&systemEnvironmentTask{provider: p},
		&kernelCmdlineTask{provider: p},
		&kernelModulesListTask{provider: p},
		&kernelTaintTask{provider: p},
		&kernelSysctlGetTask{provider: p},
		&kernelDmesgTask{provider: p},
		&processListTask{provider: p},
		&processGetTask{provider: p},
		&processTreeTask{provider: p},
		&processOpenFilesTask{provider: p},
		&processLimitsTask{provider: p},
		&processEnvironmentTask{provider: p},
		&processThreadsTask{provider: p},
		&processStackTask{provider: p},
		&memoryInfoTask{provider: p},
		&memoryVmstatTask{provider: p},
		&cpuInfoTask{provider: p},
		&cpuStatsTask{provider: p},
		&loadInfoTask{provider: p},
		&pressureGetTask{provider: p},
		&swapListTask{provider: p},
		&filesystemListTask{provider: p},
		&filesystemInodesTask{provider: p},
		&filesystemMountsTask{provider: p},
		&filesystemStatTask{provider: p},
		&filesystemReadTask{provider: p},
		&filesystemListDirectoryTask{provider: p},
		&filesystemSpaceConsumersTask{provider: p},
		&blockdeviceListTask{provider: p},
		&blockdeviceStatsTask{provider: p},
		&socketListTask{provider: p},
		&socketGetTask{provider: p},
		&interfaceListTask{provider: p},
		&routeListTask{provider: p},
		&routeLookupTask{provider: p},
		&ruleListTask{provider: p},
		&neighborListTask{provider: p},
		&networkStatsTask{provider: p},
		&dnsConfigTask{provider: p},
		&dnsResolveTask{provider: p},
		&nftablesListTask{provider: p},
		&nftablesTableGetTask{provider: p},
		&nftablesRulesetTask{provider: p},
		&iptablesRulesTask{provider: p},
		&firewalldStatusTask{provider: p},
		&firewalldZoneGetTask{provider: p},
		&systemdUnitListTask{provider: p},
		&systemdUnitStatusTask{provider: p},
		&systemdUnitShowTask{provider: p},
		&systemdUnitDefinitionTask{provider: p},
		&systemdUnitDependenciesTask{provider: p},
		&systemdUnitLogsTask{provider: p},
		&journalQueryTask{provider: p},
		&journalBootsTask{provider: p},
		&serviceFailedTask{provider: p},
		&userListTask{provider: p},
		&groupListTask{provider: p},
		&userGetTask{provider: p},
		&capabilitiesGetTask{provider: p},
		&selinuxStatusTask{provider: p},
		&apparmorStatusTask{provider: p},
		&cgroupListTask{provider: p},
		&cgroupGetTask{provider: p},
		&cgroupProcessesTask{provider: p},
		&namespaceListTask{provider: p},
		&packageGetTask{provider: p},
		&packageListTask{provider: p},
		&coreDumpListTask{provider: p},
		&coreDumpInfoTask{provider: p},
		&loginSessionsTask{provider: p},
		&loginHistoryTask{provider: p},
	}
}

func capabilityDescription(name string) string {
	descriptions := map[string]string{
		"linux.system.info":                 "Returns basic host information: hostname, OS/distribution, kernel, architecture, virtualization, boot ID",
		"linux.system.uptime":               "Returns uptime, boot time and load averages",
		"linux.system.time":                 "Returns current system time, timezone, and NTP status",
		"linux.system.environment":          "Returns selected system-level environment information with secret redaction",
		"linux.kernel.cmdline":              "Returns the kernel boot command line",
		"linux.kernel.modules.list":         "Lists loaded kernel modules and basic metadata",
		"linux.kernel.taint":                "Returns kernel taint status and decoded taint flags",
		"linux.kernel.sysctl.get":           "Reads one or more sysctl values (explicit keys only)",
		"linux.kernel.dmesg":                "Retrieves kernel ring-buffer messages with optional filtering",
		"linux.process.list":                "Structured process listing: PID, PPID, user, state, CPU, memory, command, start time",
		"linux.process.get":                 "Detailed information for one process from /proc",
		"linux.process.tree":                "Returns process hierarchy similar to pstree",
		"linux.process.open_files":          "Lists open files/file descriptors for a process",
		"linux.process.limits":              "Returns /proc/<pid>/limits",
		"linux.process.environment":         "Retrieves a process environment with mandatory secret redaction",
		"linux.process.threads":             "Lists threads belonging to a process with basic state/CPU info",
		"linux.process.stack":               "Reads kernel-visible process/thread stack where permitted",
		"linux.memory.info":                 "Memory/swap information from /proc/meminfo",
		"linux.memory.vmstat":               "VM/kernel memory counters useful for paging/swapping diagnosis",
		"linux.cpu.info":                    "CPU topology/model/core/thread information",
		"linux.cpu.stats":                   "CPU utilization/counters including user/system/iowait/steal",
		"linux.load.info":                   "Load average plus runnable/blocked task information",
		"linux.pressure.get":                "Reads Linux PSI pressure information for CPU, memory and I/O",
		"linux.swap.list":                   "Lists swap devices/files, sizes, priorities and usage",
		"linux.filesystem.list":             "Mounted filesystems, type, mount point, options, capacity and utilization",
		"linux.filesystem.inodes":           "Filesystem inode utilization",
		"linux.filesystem.mounts":           "Detailed mount tree and mount options",
		"linux.filesystem.stat":             "Metadata for a file/path: existence, type, owner, mode, size, timestamps",
		"linux.filesystem.read":             "Reads a text file with path/size restrictions and secret protection",
		"linux.filesystem.list_directory":   "Lists directory contents and metadata without modifying anything",
		"linux.filesystem.space_consumers":  "Identifies large files/directories under a constrained path",
		"linux.blockdevice.list":            "Lists disks, partitions, device mapper relationships, sizes and mount points",
		"linux.blockdevice.stats":           "Returns block-device I/O statistics",
		"linux.socket.list":                 "Lists listening and established sockets similar to ss",
		"linux.socket.get":                  "Detailed information about a particular socket/connection",
		"linux.network.interface.list":      "Lists interfaces, addresses, state, MTU, MAC, counters",
		"linux.network.route.list":          "Returns routing tables including policy routing table information",
		"linux.network.route.lookup":        "Performs a non-mutating kernel route lookup equivalent to ip route get",
		"linux.network.rule.list":           "Lists policy-routing rules (ip rule)",
		"linux.network.neighbor.list":       "Returns ARP/NDP neighbor table",
		"linux.network.stats":               "Protocol/network statistics similar to ss -s",
		"linux.network.dns.config":          "Returns resolver configuration",
		"linux.network.dns.resolve":         "Performs a DNS lookup using the host's resolver",
		"linux.firewall.nftables.list":      "Lists all nftables tables, chains, and rules from the kernel netlink interface for firewall debugging and audit",
		"linux.firewall.nftables.table.get": "Retrieves detailed chains and rules for a specific nftables table identified by name and address family",
		"linux.firewall.nftables.ruleset":   "Returns nftables ruleset in structured form using netlink where available, with procfs fallback for restricted environments",
		"linux.firewall.iptables.rules":     "Returns iptables/ip6tables rules and counters without modification",
		"linux.firewall.firewalld.status":   "Returns firewalld state and active zones if installed",
		"linux.firewall.firewalld.zone.get": "Returns configuration/effective rules for a firewalld zone",
		"linux.systemd.unit.list":           "Lists systemd units and their state",
		"linux.systemd.unit.status":         "Detailed status for a systemd unit including PID, state, exit status",
		"linux.systemd.unit.show":           "Structured systemd properties equivalent to systemctl show",
		"linux.systemd.unit.definition":     "Returns the effective unit definition including drop-ins",
		"linux.systemd.unit.dependencies":   "Returns unit dependency relationships",
		"linux.systemd.unit.logs":           "Retrieves journal entries associated with a systemd unit",
		"linux.journal.query":               "General structured journald query",
		"linux.journal.boots":               "Lists boots known to journald",
		"linux.service.failed":              "Returns failed systemd services/units",
		"linux.user.list":                   "Lists local users/accounts",
		"linux.group.list":                  "Lists groups and membership",
		"linux.user.get":                    "Account information and group membership for a user",
		"linux.security.capabilities.get":   "Returns Linux capabilities for a process/executable",
		"linux.security.selinux.status":     "Returns SELinux mode/policy/status",
		"linux.security.apparmor.status":    "Returns AppArmor status and loaded profiles",
		"linux.cgroup.list":                 "Lists cgroups and hierarchy",
		"linux.cgroup.get":                  "Returns controllers, limits and current usage for a cgroup",
		"linux.cgroup.processes":            "Lists processes belonging to a cgroup",
		"linux.namespace.list":              "Returns namespace IDs/types for a process",
		"linux.package.get":                 "Query whether a package is installed and return its version",
		"linux.package.list":                "Lists installed packages, optionally filtered",
		"linux.core_dump.list":              "Lists known systemd-coredump entries",
		"linux.core_dump.info":              "Metadata associated with a particular core dump",
		"linux.login.sessions":              "Current login/session information",
		"linux.login.history":               "Recent login/reboot/shutdown history",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Linux capability"
}
