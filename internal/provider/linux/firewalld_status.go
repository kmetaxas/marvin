package linux

import (
	"context"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// firewalldStatusTask implements the linux.firewall.firewalld.status capability.
type firewalldStatusTask struct{ provider *Provider }

const firewalldStatusSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.firewall.firewalld.status Parameters",
  "description": "Parameters for linux.firewall.firewalld.status."
}`

func (t *firewalldStatusTask) Name() string       { return "linux.firewall.firewalld.status" }
func (t *firewalldStatusTask) JSONSchema() string { return firewalldStatusSchema }

func (t *firewalldStatusTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("firewall.firewalld.status starting", "capability", t.Name())

	status, err := readFirewalldStatus(t.provider.CurrentReader())
	if err != nil {
		slog.Info("firewall.firewalld.status failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("firewall.firewalld.status succeeded", "capability", t.Name())
	return common.SuccessResult(status), nil
}

var _ task.Task = (*firewalldStatusTask)(nil)

// readFirewalldStatus inspects the local filesystem to determine whether
// firewalld is installed and running, its default zone, and the configured
// zones. It never shells out to the firewalld binary or talks to D-Bus.
func readFirewalldStatus(r *procfs.Reader) (map[string]any, error) {
	installed := r.Exists("usr", "bin", "firewalld") || r.Exists("usr", "sbin", "firewalld")

	running := r.Exists("run", "firewalld", "firewalld.pid") || r.Exists("var", "run", "firewalld.pid")

	pid := 0
	for _, elem := range [][]string{
		{"run", "firewalld", "firewalld.pid"},
		{"var", "run", "firewalld.pid"},
	} {
		if p, err := r.ReadInt(elem...); err == nil && p > 0 {
			pid = p
			break
		}
	}

	defaultZone := readFirewalldDefaultZone(r)
	zones := readFirewalldZones(r)

	return map[string]any{
		"installed":    installed,
		"running":      running,
		"pid":          pid,
		"default_zone": defaultZone,
		"zones":        zones,
		"version":      "unknown",
	}, nil
}

// readFirewalldDefaultZone parses DefaultZone= from /etc/firewalld/firewalld.conf.
func readFirewalldDefaultZone(r *procfs.Reader) string {
	data, err := r.ReadFileString("etc", "firewalld", "firewalld.conf")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "DefaultZone=") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "DefaultZone="))
	}
	return ""
}

// readFirewalldZones lists configured zones by scanning /etc/firewalld/zones/
// for *.xml files.
func readFirewalldZones(r *procfs.Reader) []string {
	names, err := r.ReadDirNames("etc", "firewalld", "zones")
	if err != nil {
		return nil
	}
	var zones []string
	for _, name := range names {
		if !strings.HasSuffix(name, ".xml") {
			continue
		}
		zones = append(zones, strings.TrimSuffix(name, ".xml"))
	}
	return zones
}
