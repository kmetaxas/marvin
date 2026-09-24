package metadata

import (
	"maps"
	"net"
	"os"
	"runtime"

	"github.com/marvin-agent/marvin/pkg/proto/marvin"
)

type Metadata struct {
	Name        string `yaml:"name" json:"name"`
	Cluster     string `yaml:"cluster" json:"cluster"`
	Region      string `yaml:"region" json:"region"`
	Environment string `yaml:"environment" json:"environment"`

	// Host metadata (matches proto HostMetadata).
	Hostname         string `yaml:"hostname" json:"hostname"`
	LocalIP          string `yaml:"local_ip" json:"local_ip"`
	Provider         string `yaml:"provider" json:"provider"`
	AvailabilityZone string `yaml:"availability_zone" json:"availability_zone"`
	VMID             string `yaml:"vm_id" json:"vm_id"`
	OS               string `yaml:"os" json:"os"`
	OSVersion        string `yaml:"os_version" json:"os_version"`
	Arch             string `yaml:"arch" json:"arch"`

	Labels map[string]string `yaml:"labels" json:"labels"`
}

func (m Metadata) ToProtoMap() map[string]string {
	out := map[string]string{
		"name":        m.Name,
		"cluster":     m.Cluster,
		"region":      m.Region,
		"environment": m.Environment,
	}
	maps.Copy(out, m.Labels)

	return out
}

// AutodetectDefaults populates empty metadata fields with system-detected or
// sensible fallback values. Fields explicitly set by the user are never overwritten.
func (m *Metadata) AutodetectDefaults() error {
	if m.Hostname == "" {
		m.Hostname = detectHostname()
	}
	if m.LocalIP == "" {
		m.LocalIP = detectLocalIP()
	}
	if m.OS == "" {
		m.OS = detectOS()
	}
	if m.OSVersion == "" {
		m.OSVersion = detectOSVersion()
	}
	if m.Arch == "" {
		m.Arch = detectArch()
	}
	return nil
}

func detectHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func detectLocalIP() string {
	// Attempt to find the local IP by creating a UDP connection to a public
	// address. No packets are sent; the kernel just binds to an interface.
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "127.0.0.1"
	}
	return addr.IP.String()
}

func detectOS() string {
	return runtime.GOOS
}

func detectOSVersion() string {
	// Standard Go does not expose a portable OS version API.
	// Returning GOOS as a reasonable placeholder for now.
	return runtime.GOOS
}

func detectArch() string {
	goarch := runtime.GOARCH
	// Normalise common Go arch names to more user-friendly ones.
	switch goarch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return goarch
	}
}

// ToProto converts the metadata into a proto HostMetadata message.
func (m Metadata) ToProto() *marvin.HostMetadata {
	return &marvin.HostMetadata{
		Hostname:         m.Hostname,
		LocalIp:          m.LocalIP,
		Provider:         m.Provider,
		Region:           m.Region,
		AvailabilityZone: m.AvailabilityZone,
		VmId:             m.VMID,
		Os:               m.OS,
		OsVersion:        m.OSVersion,
		Arch:             m.Arch,
	}
}

// ToLabels returns the labels encoded as proto-compatible key=value strings.
// Labels are also exposed as a []string for the proto Register message.
func (m Metadata) ToLabels() []string {
	seen := map[string]string{}
	maps.Copy(seen, m.Labels)

	out := make([]string, 0, len(seen))
	for key, value := range seen {
		out = append(out, key+"="+value)
	}
	return out
}
