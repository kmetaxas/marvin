package linux

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"strings"

	"github.com/marvin-agent/marvin/internal/provider/common"
	"github.com/marvin-agent/marvin/internal/provider/linux/procfs"
	"github.com/marvin-agent/marvin/internal/task"
)

// firewalldZoneGetTask implements the linux.firewall.firewalld.zone.get capability.
type firewalldZoneGetTask struct{ provider *Provider }

const firewalldZoneGetSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "linux.firewall.firewalld.zone.get Parameters",
  "description": "Parameters for linux.firewall.firewalld.zone.get.",
  "properties": {
    "zone": {
      "type": "string",
      "description": "Firewalld zone name to query"
    }
  },
  "required": ["zone"]
}`

func (t *firewalldZoneGetTask) Name() string       { return "linux.firewall.firewalld.zone.get" }
func (t *firewalldZoneGetTask) JSONSchema() string { return firewalldZoneGetSchema }

func (t *firewalldZoneGetTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	slog.Info("firewall.firewalld.zone.get starting", "capability", t.Name())

	zone, err := common.RequireString(params, "zone")
	if err != nil {
		return common.TaskFailure(err)
	}
	if err := validateZoneName(zone); err != nil {
		return common.TaskFailure(err)
	}

	detail, err := readFirewalldZone(t.provider.CurrentReader(), zone)
	if err != nil {
		slog.Info("firewall.firewalld.zone.get failed", "capability", t.Name(), "error", err)
		return common.TaskFailure(err)
	}

	slog.Info("firewall.firewalld.zone.get succeeded", "capability", t.Name(), "zone", zone)
	return common.SuccessResult(detail), nil
}

var _ task.Task = (*firewalldZoneGetTask)(nil)

// validateZoneName rejects zone names that could escape the zones directory.
func validateZoneName(zone string) error {
	if strings.Contains(zone, "/") || strings.Contains(zone, "\\") || zone == "." || zone == ".." {
		return fmt.Errorf("invalid zone name: %q", zone)
	}
	return nil
}

// firewalldZoneXML mirrors the on-disk firewalld zone definition format.
type firewalldZoneXML struct {
	XMLName     xml.Name                `xml:"zone"`
	Short       string                  `xml:"short"`
	Description string                  `xml:"description"`
	Services    []firewalldServiceXML   `xml:"service"`
	Ports       []firewalldPortXML      `xml:"port"`
	Sources     []firewalldSourceXML    `xml:"source"`
	Interfaces  []firewalldInterfaceXML `xml:"interface"`
	Forward     *struct{}               `xml:"forward"`
	Masquerade  *struct{}               `xml:"masquerade"`
}

type firewalldServiceXML struct {
	Name string `xml:"name,attr"`
}

type firewalldPortXML struct {
	Port     string `xml:"port,attr"`
	Protocol string `xml:"protocol,attr"`
}

type firewalldSourceXML struct {
	Address string `xml:"address,attr"`
}

type firewalldInterfaceXML struct {
	Name string `xml:"name,attr"`
}

// readFirewalldZone reads and parses a single zone definition file from
// /etc/firewalld/zones/<zone>.xml.
func readFirewalldZone(r *procfs.Reader, zone string) (map[string]any, error) {
	data, err := r.ReadFileString("etc", "firewalld", "zones", zone+".xml")
	if err != nil {
		return nil, fmt.Errorf("read zone %q: %w", zone, err)
	}

	var z firewalldZoneXML
	if err := xml.Unmarshal([]byte(data), &z); err != nil {
		return nil, fmt.Errorf("parse zone %q: %w", zone, err)
	}

	services := make([]string, 0, len(z.Services))
	for _, s := range z.Services {
		if s.Name != "" {
			services = append(services, s.Name)
		}
	}

	ports := make([]map[string]string, 0, len(z.Ports))
	for _, p := range z.Ports {
		ports = append(ports, map[string]string{
			"protocol": p.Protocol,
			"port":     p.Port,
		})
	}

	sources := make([]string, 0, len(z.Sources))
	for _, s := range z.Sources {
		if s.Address != "" {
			sources = append(sources, s.Address)
		}
	}

	interfaces := make([]string, 0, len(z.Interfaces))
	for _, i := range z.Interfaces {
		if i.Name != "" {
			interfaces = append(interfaces, i.Name)
		}
	}

	return map[string]any{
		"name":        zone,
		"short":       z.Short,
		"description": z.Description,
		"services":    services,
		"ports":       ports,
		"sources":     sources,
		"interfaces":  interfaces,
		"forward":     z.Forward != nil,
		"masquerade":  z.Masquerade != nil,
	}, nil
}
