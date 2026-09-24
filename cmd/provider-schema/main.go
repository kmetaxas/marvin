package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/invopop/jsonschema"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/marvin-agent/marvin/internal/provider"
	"github.com/marvin-agent/marvin/internal/provider/azure"
	"github.com/marvin-agent/marvin/internal/provider/kafka"
	"github.com/marvin-agent/marvin/internal/provider/kubernetes"
	"github.com/marvin-agent/marvin/internal/provider/linux"
	logprovider "github.com/marvin-agent/marvin/internal/provider/log"
	"github.com/marvin-agent/marvin/internal/provider/network"
	"github.com/marvin-agent/marvin/internal/provider/prometheus"
	"github.com/marvin-agent/marvin/pkg/capability"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	return runWithArgs(os.Args[1:], os.Stdout)
}

func runWithArgs(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("provider-schema", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outputFormat := fs.String("o", "text", "output format (text)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	remaining := fs.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("usage: provider-schema <provider-name> [-o text]\n\navailable providers: %s", availableProviders())
	}

	providerName := remaining[0]
	pf, ok := providerFactories[providerName]
	if !ok {
		return fmt.Errorf("unknown provider %q\n\navailable providers: %s", providerName, availableProviders())
	}

	prov := pf.create()

	switch *outputFormat {
	case "text":
		return outputText(out, prov, pf.configType)
	default:
		return fmt.Errorf("unsupported output format %q", *outputFormat)
	}
}

type factory struct {
	create     func() provider.Provider
	configType any
}

var providerFactories = map[string]factory{
	"linux": {
		create:     func() provider.Provider { return linux.NewProvider(config.LinuxConfig{}) },
		configType: config.LinuxConfig{},
	},
	"network": {
		create:     func() provider.Provider { return network.NewProvider() },
		configType: nil,
	},
	"azure": {
		create:     func() provider.Provider { return azure.NewProvider(config.AzureConfig{}) },
		configType: config.AzureConfig{},
	},
	"kubernetes": {
		create:     func() provider.Provider { return kubernetes.NewProvider(config.KubernetesConfig{}) },
		configType: config.KubernetesConfig{},
	},
	"prometheus": {
		create:     func() provider.Provider { return prometheus.NewProvider(config.PrometheusConfig{}) },
		configType: config.PrometheusConfig{},
	},
	"kafka": {
		create:     func() provider.Provider { return kafka.NewProvider(config.KafkaConfig{}) },
		configType: config.KafkaConfig{},
	},
	"log": {
		create:     func() provider.Provider { return logprovider.NewProvider(config.GraylogConfig{}) },
		configType: config.GraylogConfig{},
	},
	"os": {
		create: func() provider.Provider {
			return provider.BaseProvider{
				ProviderName: "os",
				ProviderCapabilities: []capability.Capability{
					{Name: "os.process.list", Version: "v1", Description: "Lists processes", Provider: "os"},
					{Name: "os.file.read", Version: "v1", Description: "Reads files", Provider: "os"},
				},
			}
		},
		configType: nil,
	},
}

func availableProviders() string {
	names := make([]string, 0, len(providerFactories))
	for name := range providerFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func outputText(out io.Writer, prov provider.Provider, configType any) error {
	_, _ = fmt.Fprintf(out, "=== Provider: %s ===\n\n", prov.Name())

	if configType != nil {
		schema := jsonschema.Reflect(configType)
		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal config schema: %w", err)
		}
		_, _ = fmt.Fprintf(out, "--- Configuration JSON Schema ---\n")
		_, _ = fmt.Fprintf(out, "%s\n", string(schemaJSON))
		_, _ = fmt.Fprintf(out, "\n")
	} else {
		_, _ = fmt.Fprintf(out, "--- Configuration JSON Schema ---\n")
		_, _ = fmt.Fprintf(out, "{}\n")
		_, _ = fmt.Fprintf(out, "\n")
	}

	caps := prov.Capabilities()
	_, _ = fmt.Fprintf(out, "--- Capabilities ---\n")
	_, _ = fmt.Fprintf(out, "Total: %d capabilities\n\n", len(caps))

	capNames := make([]string, len(caps))
	for i, cap := range caps {
		capNames[i] = cap.Name
	}
	capNamesJSON, _ := json.Marshal(capNames)
	_, _ = fmt.Fprintf(out, "Capability names (Resource Type list):\n%s\n\n", string(capNamesJSON))

	_, _ = fmt.Fprintf(out, "Capabilities with details:\n")
	for i, cap := range caps {
		_, _ = fmt.Fprintf(out, "%d. %s\n", i+1, cap.Name)
		if cap.Description != "" {
			_, _ = fmt.Fprintf(out, "   Description: %s\n", cap.Description)
		}
		if cap.Version != "" {
			_, _ = fmt.Fprintf(out, "   Version: %s\n", cap.Version)
		}
		if cap.ParametersJSONSchema != "" {
			_, _ = fmt.Fprintf(out, "   Parameters JSON Schema:\n")
			var pretty map[string]any
			if err := json.Unmarshal([]byte(cap.ParametersJSONSchema), &pretty); err == nil {
				prettyJSON, _ := json.MarshalIndent(pretty, "", "  ")
				lines := strings.Split(string(prettyJSON), "\n")
				for _, line := range lines {
					_, _ = fmt.Fprintf(out, "   %s\n", line)
				}
			} else {
				for _, line := range strings.Split(cap.ParametersJSONSchema, "\n") {
					_, _ = fmt.Fprintf(out, "   %s\n", line)
				}
			}
		}
		_, _ = fmt.Fprintf(out, "\n")
	}

	return nil
}
