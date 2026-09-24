package capability

import (
	"fmt"
	"path"
	"regexp"

	"github.com/marvin-agent/marvin/pkg/proto/marvin"
	"google.golang.org/protobuf/types/known/structpb"
)

var namePattern = regexp.MustCompile(`^[a-z]+(\.[a-z_]+)+$`)

type Capability struct {
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	Description string `json:"description" yaml:"description"`
	Provider    string `json:"provider" yaml:"provider"`

	// ParametersJSONSchema is a JSON Schema string describing the expected
	// parameters for execution. The control plane validates parameters
	// against this before dispatching.
	ParametersJSONSchema string `json:"parameters_json_schema,omitempty" yaml:"parameters_json_schema,omitempty"`

	// Config is the capability-specific configuration, e.g. the configured
	// user/realm for a kerberos.tgt.request capability.
	Config map[string]any `json:"config,omitempty" yaml:"config,omitempty"`

	// ConfigSummary is an optional human-readable summary of the current config.
	ConfigSummary string `json:"config_summary,omitempty" yaml:"config_summary,omitempty"`

	// Enabled indicates whether this capability is enabled and ready to accept
	// commands.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// UseCases describes troubleshooting scenarios where this capability is useful.
	UseCases []string `json:"use_cases,omitempty" yaml:"use_cases,omitempty"`

	// Aliases are alternative names and abbreviations for discovery.
	Aliases []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`

	// Tags are structured taxonomy labels for filtering and grouping.
	Tags []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid capability name %q", name)
	}

	return nil
}

func (c Capability) Validate() error {
	if err := ValidateName(c.Name); err != nil {
		return err
	}

	return nil
}

func (c Capability) String() string {
	return c.Name
}

// IsPattern returns true if the string contains a wildcard character.
func IsPattern(s string) bool {
	return regexp.MustCompile(`\*`).MatchString(s)
}

// Match evaluates whether a capability name matches a pattern using
// path.Match semantics (single * matches any sequence of characters).
func Match(name, pattern string) (bool, error) {
	return path.Match(pattern, name)
}

// ValidatePattern checks that a pattern string is valid (non-empty, contains
// at least one non-wildcard character, and uses only allowed characters).
func ValidatePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern must not be empty")
	}
	if pattern != "*" && regexp.MustCompile(`^\*+$`).MatchString(pattern) {
		return fmt.Errorf("pattern must contain at least one non-wildcard character")
	}
	if !regexp.MustCompile(`^[a-z0-9_*\.]+$`).MatchString(pattern) {
		return fmt.Errorf("pattern %q contains invalid characters", pattern)
	}
	return nil
}

// ToProto converts the capability into a proto CapabilityManifest message.
func (c Capability) ToProto() *marvin.CapabilityManifest {
	manifest := &marvin.CapabilityManifest{
		Name:                 c.Name,
		Description:          c.Description,
		Enabled:              c.Enabled,
		ParametersJsonSchema: c.ParametersJSONSchema,
		ConfigSummary:        c.ConfigSummary,
		UseCases:             c.UseCases,
		Aliases:              c.Aliases,
		Tags:                 c.Tags,
	}

	if len(c.Config) > 0 {
		if structValue, err := structpb.NewStruct(c.Config); err == nil {
			manifest.Config = structValue
		}
	}

	return manifest
}
