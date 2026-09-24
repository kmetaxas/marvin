package network

import (
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/task"
	"github.com/marvin-agent/marvin/pkg/capability"
)

// NewProvider creates a real network provider with all its tasks wired in.
func NewProvider() provider.Provider {
	dnsTask := &dnsTask{}
	pingTask := &pingTask{}
	socketTask := &socketTask{}
	tlsTask := &tlsTask{}
	tracerouteTask := &tracerouteTask{}

	return provider.BaseProvider{
		ProviderName: "network",
		ProviderCapabilities: []capability.Capability{
			{
				Name:                 dnsTask.Name(),
				Version:              "v1",
				Description:          "Performs DNS lookups for various DNS record types including A, AAAA, CNAME, TXT, SRV, and PTR records.",
				Provider:             "network",
				ParametersJSONSchema: dnsTask.JSONSchema(),
				UseCases: []string{
					"Checking whether a host can resolve a domain name",
					"Verifying DNS resolution for service discovery",
					"Diagnosing DNS misconfigurations",
					"Testing reverse DNS lookups",
				},
				Aliases: []string{"dns lookup", "name resolution", "reverse dns"},
				Tags:    []string{"dns", "resolution", "lookup", "network", "connectivity"},
			},
			{
				Name:                 pingTask.Name(),
				Version:              "v1",
				Description:          "Sends ICMP echo requests to test network reachability and measure latency.",
				Provider:             "network",
				ParametersJSONSchema: pingTask.JSONSchema(),
				UseCases: []string{
					"Testing network reachability between hosts",
					"Measuring network latency",
					"Checking if a host is online",
				},
				Aliases: []string{"icmp", "reachability", "latency test"},
				Tags:    []string{"ping", "icmp", "network", "reachability", "latency"},
			},
			{
				Name:                 socketTask.Name(),
				Version:              "v1",
				Description:          "Tests TCP/UDP socket connectivity without sending application data.",
				Provider:             "network",
				ParametersJSONSchema: socketTask.JSONSchema(),
				UseCases: []string{
					"Testing TCP/UDP port connectivity",
					"Verifying service availability on a specific port",
					"Checking firewall rules",
				},
				Aliases: []string{"port check", "connectivity test", "socket test"},
				Tags:    []string{"socket", "tcp", "udp", "port", "network", "connectivity"},
			},
			{
				Name:                 tlsTask.Name(),
				Version:              "v1",
				Description:          "Performs TLS handshakes and returns certificate chain details.",
				Provider:             "network",
				ParametersJSONSchema: tlsTask.JSONSchema(),
				UseCases: []string{
					"Checking TLS certificate validity and expiry",
					"Verifying TLS handshake succeeds",
					"Inspecting certificate chain",
					"Checking supported TLS versions and ciphers",
				},
				Aliases: []string{"certificate check", "ssl", "tls check"},
				Tags:    []string{"tls", "ssl", "certificate", "security", "network"},
			},
			{
				Name:                 tracerouteTask.Name(),
				Version:              "v1",
				Description:          "Traces the network route to a target host by sending probes with incrementing TTL values and reporting each intermediate router hop, latency, and path completion status for network path diagnosis",
				Provider:             "network",
				ParametersJSONSchema: tracerouteTask.JSONSchema(),
				UseCases: []string{
					"Identifying routing paths and network topology",
					"Diagnosing network latency at each hop",
					"Detecting packet loss or routing loops",
					"Verifying traffic flows through expected gateways",
				},
				Aliases: []string{"trace route", "path trace", "network path", "hop trace"},
				Tags:    []string{"traceroute", "network", "routing", "path", "latency", "icmp", "udp", "diagnostics"},
			},
		},
		Tasks: map[string]task.Task{
			dnsTask.Name():        dnsTask,
			pingTask.Name():       pingTask,
			socketTask.Name():     socketTask,
			tlsTask.Name():        tlsTask,
			tracerouteTask.Name(): tracerouteTask,
		},
	}
}
