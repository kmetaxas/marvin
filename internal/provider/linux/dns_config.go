package linux

import (
	"context"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// dnsConfigTask implements the linux.network.dns.config capability.
type dnsConfigTask struct{ provider *Provider }

const dnsConfigSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "DNS Config Parameters",
  "description": "Parameters for retrieving resolver configuration."
}`

func (t *dnsConfigTask) Name() string       { return "linux.network.dns.config" }
func (t *dnsConfigTask) JSONSchema() string { return dnsConfigSchema }

func (t *dnsConfigTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	_ = ctx
	_ = params
	slog.Info("network.dns.config starting", "capability", t.Name())

	reader := t.provider.CurrentReader()
	cfg, err := parseResolvConf(reader)
	if err != nil {
		slog.Info("network.dns.config failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("network.dns.config succeeded", "capability", t.Name())
	return common.SuccessResult(cfg), nil
}

var _ task.Task = (*dnsConfigTask)(nil)

func parseResolvConf(r *procfs.Reader) (map[string]any, error) {
	data, err := r.ReadFileString("etc", "resolv.conf")
	if err != nil {
		return nil, err
	}

	cfg := map[string]any{
		"nameservers": []string{},
		"search":      []string{},
		"options":     []string{},
		"sortlist":    []string{},
	}

	var nameservers, search, options, sortlist []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "nameserver":
			if len(fields) >= 2 {
				nameservers = append(nameservers, fields[1])
			}
		case "domain":
			if len(fields) >= 2 {
				cfg["domain"] = fields[1]
			}
		case "search":
			search = append(search, fields[1:]...)
		case "options":
			options = append(options, fields[1:]...)
		case "sortlist":
			sortlist = append(sortlist, fields[1:]...)
		}
	}

	cfg["nameservers"] = nameservers
	cfg["search"] = search
	cfg["options"] = options
	cfg["sortlist"] = sortlist

	return cfg, nil
}
