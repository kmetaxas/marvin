package linux

// ProcessSummary represents a process in list results.
type ProcessSummary struct {
	PID        int     `json:"pid"`
	PPID       int     `json:"ppid"`
	User       string  `json:"user"`
	State      string  `json:"state"`
	CPUPercent float64 `json:"cpu_percent,omitempty"`
	MemPercent float64 `json:"mem_percent,omitempty"`
	Command    string  `json:"command"`
	StartTime  string  `json:"start_time,omitempty"`
}

// ProcessDetail represents detailed process information.
type ProcessDetail struct {
	ProcessSummary
	Executable   string            `json:"executable,omitempty"`
	CWD          string            `json:"cwd,omitempty"`
	Threads      int               `json:"threads"`
	Memory       MemoryInfo        `json:"memory"`
	Namespaces   map[string]string `json:"namespaces,omitempty"`
	Cgroups      []string          `json:"cgroups,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
}

// MemoryInfo represents process memory usage.
type MemoryInfo struct {
	RSS    uint64 `json:"rss_bytes"`
	VMS    uint64 `json:"vms_bytes"`
	Swap   uint64 `json:"swap_bytes,omitempty"`
	Data   uint64 `json:"data_bytes,omitempty"`
	Stack  uint64 `json:"stack_bytes,omitempty"`
	Text   uint64 `json:"text_bytes,omitempty"`
	Shared uint64 `json:"shared_bytes,omitempty"`
}

// SocketInfo represents a network socket.
type SocketInfo struct {
	Protocol      string `json:"protocol"`
	LocalAddress  string `json:"local_address"`
	LocalPort     int    `json:"local_port"`
	RemoteAddress string `json:"remote_address,omitempty"`
	RemotePort    int    `json:"remote_port,omitempty"`
	State         string `json:"state"`
	PID           int    `json:"pid,omitempty"`
	UID           int    `json:"uid,omitempty"`
	Inode         uint64 `json:"inode"`
}

// InterfaceInfo represents a network interface.
type InterfaceInfo struct {
	Name       string   `json:"name"`
	Index      int      `json:"index"`
	MTU        int      `json:"mtu"`
	MACAddress string   `json:"mac_address,omitempty"`
	Flags      []string `json:"flags"`
	State      string   `json:"state"`
	Addresses  []string `json:"addresses,omitempty"`
	RxBytes    uint64   `json:"rx_bytes,omitempty"`
	TxBytes    uint64   `json:"tx_bytes,omitempty"`
}

// MountInfo represents a filesystem mount.
type MountInfo struct {
	Source         string   `json:"source"`
	Target         string   `json:"target"`
	FilesystemType string   `json:"filesystem_type"`
	Options        []string `json:"options"`
	TotalBytes     uint64   `json:"total_bytes,omitempty"`
	FreeBytes      uint64   `json:"free_bytes,omitempty"`
	UsedBytes      uint64   `json:"used_bytes,omitempty"`
	UsedPercent    float64  `json:"used_percent,omitempty"`
}

// SystemdUnit represents a systemd unit.
type SystemdUnit struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
	Type        string `json:"type,omitempty"`
}

// JournalEntry represents a journald log entry.
type JournalEntry struct {
	Timestamp  string            `json:"timestamp"`
	Message    string            `json:"message"`
	Priority   string            `json:"priority"`
	Unit       string            `json:"unit,omitempty"`
	Identifier string            `json:"identifier,omitempty"`
	PID        int               `json:"pid,omitempty"`
	UID        int               `json:"uid,omitempty"`
	BootID     string            `json:"boot_id,omitempty"`
	Fields     map[string]string `json:"fields,omitempty"`
}

// KernelModule represents a loaded kernel module.
type KernelModule struct {
	Name      string   `json:"name"`
	Size      uint64   `json:"size"`
	Instances int      `json:"instances"`
	Depends   []string `json:"depends,omitempty"`
	State     string   `json:"state"`
	Memory    uint64   `json:"memory,omitempty"`
}

// RouteInfo represents a routing table entry.
type RouteInfo struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Source      string `json:"source,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Table       int    `json:"table,omitempty"`
}

// NeighborInfo represents an ARP/NDP neighbor table entry.
type NeighborInfo struct {
	Address string `json:"address"`
	MAC     string `json:"mac_address,omitempty"`
	State   string `json:"state,omitempty"`
	Device  string `json:"device,omitempty"`
	Type    string `json:"type"`
}

// CgroupInfo represents a cgroup.
type CgroupInfo struct {
	Path        string            `json:"path"`
	Controllers []string          `json:"controllers,omitempty"`
	Limits      map[string]string `json:"limits,omitempty"`
	Usage       map[string]string `json:"usage,omitempty"`
	Processes   []int             `json:"processes,omitempty"`
}
