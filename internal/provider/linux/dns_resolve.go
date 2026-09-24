package linux

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/task"
)

// dnsResolveTask implements the linux.network.dns.resolve capability.
type dnsResolveTask struct{ provider *Provider }

const dnsResolveSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "DNS Resolve Parameters",
  "description": "Parameters for performing a DNS lookup using the host's resolver.",
  "properties": {
    "target": {
      "type": "string",
      "description": "Domain name to resolve, or IP address for reverse lookup."
    },
    "record_type": {
      "type": "string",
      "description": "DNS record type to query.",
      "enum": ["A", "AAAA", "CNAME", "TXT", "PTR"],
      "default": "A"
    }
  },
  "required": ["target"]
}`

func (t *dnsResolveTask) Name() string       { return "linux.network.dns.resolve" }
func (t *dnsResolveTask) JSONSchema() string { return dnsResolveSchema }

func (t *dnsResolveTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	start := time.Now()

	target, err := common.RequireString(params, "target")
	if err != nil {
		return common.TaskFailure(err)
	}

	recordType, _ := params["record_type"].(string)
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	if recordType == "" {
		recordType = "A"
	}

	slog.Info("network.dns.resolve starting", "capability", t.Name(), "target", target, "record_type", recordType)

	resolver := &net.Resolver{}
	answers, err := resolveDNS(ctx, resolver, target, recordType)
	if err != nil {
		slog.Info("network.dns.resolve failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	result := map[string]any{
		"target":      target,
		"record_type": recordType,
		"answers":     answers,
		"count":       len(answers),
		"duration_ms": time.Since(start).Milliseconds(),
	}
	slog.Info("network.dns.resolve succeeded", "capability", t.Name(), "count", len(answers))
	return common.SuccessResult(result), nil
}

var _ task.Task = (*dnsResolveTask)(nil)

type dnsRecord struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl,omitempty"`
}

func resolveDNS(ctx context.Context, r *net.Resolver, target, recordType string) ([]dnsRecord, error) {
	switch recordType {
	case "A":
		ips, err := r.LookupIPAddr(ctx, target)
		if err != nil {
			return nil, err
		}
		var records []dnsRecord
		for _, ip := range ips {
			if ip.IP.To4() != nil {
				records = append(records, dnsRecord{Type: "A", Value: ip.IP.String()})
			}
		}
		return records, nil
	case "AAAA":
		ips, err := r.LookupIPAddr(ctx, target)
		if err != nil {
			return nil, err
		}
		var records []dnsRecord
		for _, ip := range ips {
			if ip.IP.To4() == nil {
				records = append(records, dnsRecord{Type: "AAAA", Value: ip.IP.String()})
			}
		}
		return records, nil
	case "CNAME":
		cname, err := r.LookupCNAME(ctx, target)
		if err != nil {
			return nil, err
		}
		return []dnsRecord{{Type: "CNAME", Value: cname}}, nil
	case "TXT":
		txts, err := r.LookupTXT(ctx, target)
		if err != nil {
			return nil, err
		}
		var records []dnsRecord
		for _, txt := range txts {
			records = append(records, dnsRecord{Type: "TXT", Value: txt})
		}
		return records, nil
	case "PTR":
		names, err := r.LookupAddr(ctx, target)
		if err != nil {
			return nil, err
		}
		var records []dnsRecord
		for _, name := range names {
			records = append(records, dnsRecord{Type: "PTR", Value: name})
		}
		return records, nil
	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}
}
