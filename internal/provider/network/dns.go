package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/task"
)

// dnsTask performs DNS lookups for various record types.
type dnsTask struct{}

const dnsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "DNS Lookup Parameters",
  "description": "Parameters for performing a DNS lookup.",
  "properties": {
    "target": {
      "type": "string",
      "description": "Domain name or IP address to query. For PTR lookups, provide an IP address."
    },
    "record_type": {
      "type": "string",
      "description": "DNS record type to query.",
      "enum": ["A", "AAAA", "CNAME", "TXT", "SRV", "PTR"],
      "default": "A"
    },
    "protocol": {
      "type": "string",
      "description": "Network protocol for custom resolver (e.g., 'udp', 'tcp').",
      "enum": ["udp", "tcp"]
    },
    "server": {
      "type": "string",
      "description": "Custom DNS resolver address (e.g., '8.8.8.8:53')."
    }
  },
  "required": ["target"]
}`

func (d *dnsTask) Name() string { return "network.dns.lookup" }

func (d *dnsTask) JSONSchema() string { return dnsSchema }

func (d *dnsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	start := time.Now()
	result := task.Result{
		Success:   true,
		Timestamp: start,
	}

	target, ok := params["target"].(string)
	if !ok || target == "" {
		result.Success = false
		result.Error = "missing required parameter: target"
		return result, nil
	}

	recordType, _ := params["record_type"].(string)
	if recordType == "" {
		recordType = "A"
	}
	recordType = strings.ToUpper(recordType)

	protocol, _ := params["protocol"].(string)
	server, _ := params["server"].(string)

	slog.Info("DNS lookup starting", "capability", d.Name(), "target", target, "record_type", recordType)
	slog.Debug("DNS lookup parameters", "capability", d.Name(), "target", target, "record_type", recordType, "protocol", protocol, "server", server)

	resolver := &net.Resolver{}
	if protocol != "" || server != "" {
		resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			dialNetwork := network
			if protocol != "" {
				dialNetwork = protocol
			}
			dialAddr := address
			if server != "" {
				dialAddr = server
			}
			return net.Dial(dialNetwork, dialAddr)
		}
	}

	var answers []dnsAnswer
	var err error

	switch recordType {
	case "A":
		answers, err = d.lookupA(ctx, resolver, target)
	case "AAAA":
		answers, err = d.lookupAAAA(ctx, resolver, target)
	case "CNAME":
		answers, err = d.lookupCNAME(ctx, resolver, target)
	case "TXT":
		answers, err = d.lookupTXT(ctx, resolver, target)
	case "SRV":
		answers, err = d.lookupSRV(ctx, resolver, target)
	case "PTR":
		answers, err = d.lookupPTR(ctx, resolver, target)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported record type: %s", recordType)
		return result, nil
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		slog.Info("DNS lookup failed", "capability", d.Name(), "target", target, "record_type", recordType, "error", result.Error)
		return result, nil
	}

	result.Data = map[string]any{
		"answers":     answers,
		"server_used": server,
		"duration_ms": time.Since(start).Milliseconds(),
	}
	slog.Info("DNS lookup succeeded", "capability", d.Name(), "target", target, "record_type", recordType, "answer_count", len(answers))
	slog.Debug("DNS lookup result", "capability", d.Name(), "target", target, "answers", answers, "server_used", server)
	return result, nil
}

type dnsAnswer struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl,omitempty"`
}

func (d *dnsTask) lookupA(ctx context.Context, r *net.Resolver, target string) ([]dnsAnswer, error) {
	ips, err := r.LookupIPAddr(ctx, target)
	if err != nil {
		return nil, err
	}
	var answers []dnsAnswer
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			answers = append(answers, dnsAnswer{Type: "A", Value: ip.IP.String()})
		}
	}
	return answers, nil
}

func (d *dnsTask) lookupAAAA(ctx context.Context, r *net.Resolver, target string) ([]dnsAnswer, error) {
	ips, err := r.LookupIPAddr(ctx, target)
	if err != nil {
		return nil, err
	}
	var answers []dnsAnswer
	for _, ip := range ips {
		if ip.IP.To4() == nil {
			answers = append(answers, dnsAnswer{Type: "AAAA", Value: ip.IP.String()})
		}
	}
	return answers, nil
}

func (d *dnsTask) lookupCNAME(ctx context.Context, r *net.Resolver, target string) ([]dnsAnswer, error) {
	cname, err := r.LookupCNAME(ctx, target)
	if err != nil {
		return nil, err
	}
	return []dnsAnswer{{Type: "CNAME", Value: cname}}, nil
}

func (d *dnsTask) lookupTXT(ctx context.Context, r *net.Resolver, target string) ([]dnsAnswer, error) {
	txts, err := r.LookupTXT(ctx, target)
	if err != nil {
		return nil, err
	}
	var answers []dnsAnswer
	for _, txt := range txts {
		answers = append(answers, dnsAnswer{Type: "TXT", Value: txt})
	}
	return answers, nil
}

func (d *dnsTask) lookupSRV(ctx context.Context, r *net.Resolver, target string) ([]dnsAnswer, error) {
	_, addrs, err := r.LookupSRV(ctx, "", "", target)
	if err != nil {
		return nil, err
	}
	var answers []dnsAnswer
	for _, addr := range addrs {
		v, _ := json.Marshal(addr)
		answers = append(answers, dnsAnswer{Type: "SRV", Value: string(v)})
	}
	return answers, nil
}

func (d *dnsTask) lookupPTR(ctx context.Context, r *net.Resolver, target string) ([]dnsAnswer, error) {
	names, err := r.LookupAddr(ctx, target)
	if err != nil {
		return nil, err
	}
	var answers []dnsAnswer
	for _, name := range names {
		answers = append(answers, dnsAnswer{Type: "PTR", Value: name})
	}
	return answers, nil
}
