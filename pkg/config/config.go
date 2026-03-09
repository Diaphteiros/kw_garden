package config

import (
	"fmt"

	"github.com/Diaphteiros/kw/pluginlib/pkg/debug"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

type GardenctlConfig struct {
	// Binary is the path to the gardenctl binary or just a name (has to be on the PATH in the latter case).
	// Defaults to 'gardenctl'.
	Binary string `json:"binary"`
	// ConfigDir contains the path to the gardenctl config directory.
	// If empty, gardenctl will use the default config dir.
	ConfigDir string `json:"configDir,omitempty"`
	// ConfigFileName contains the name of the gardenctl config file.
	// If empty, gardenctl will use the default config file name.
	ConfigFileName string `json:"configFileName,omitempty"`
	// URLs contains a list of regexes to match Gardener dashboard URLs against.
	// When the 'dashboard' subcommand is used, this list is traversed top to bottom and the first match is used.
	URLs []URLSpecification `json:"urls,omitempty"`
}

type URLSpecification struct {
	// Regex is a regular expression to match against the Gardener dashboard URL.
	// The regex must capture a 'project' and 'shoot' group.
	// If the Garden field is not set, it must also capture a 'garden' group.
	// Example: ^(https:\/\/)?dashboard\.garden\.(?P<garden>[a-zA-Z0-9-]*)\.example\.com\/namespace\/garden-(?P<project>[a-z0-9-]*)\/shoots\/(?P<shoot>[a-z0-9-]*)(\/.*)?$
	//   would match 'dashboard.garden.my-garden.example.com/namespace/garden-my-project/shoots/my-shoot' and capture 'my-garden', 'my-project', and 'my-shoot'.
	Regex string `json:"regex"`

	// Garden can be used to overwrite the garden name, if no segment of the URL matches the desired name.
	// If this is set, the value of the 'garden' group in the regex is ignored, if it exists at all.
	// Otherwise, a 'garden' group has to be captured.
	Garden string `json:"garden,omitempty"`
}

func (c *GardenctlConfig) String() string {
	if c == nil {
		return ""
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("error marshaling config: %v", err)
	}
	return string(data)
}

func (gc *GardenctlConfig) Default() error {
	if gc.Binary == "" {
		gc.Binary = "gardenctl"
	}
	return nil
}

func (c *GardenctlConfig) Validate() error {
	errs := field.ErrorList{}
	if c.Binary == "" {
		errs = append(errs, field.Required(field.NewPath("binary"), "gardenctl binary path is required"))
	}
	return errs.ToAggregate()
}

func LoadFromBytes(data []byte) (*GardenctlConfig, error) {
	cfg := &GardenctlConfig{}
	if len(data) > 0 {
		err := yaml.Unmarshal(data, cfg)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling kw_garden config: %w", err)
		}
	} else {
		debug.Debug("No kw_garden config provided, using default values")
	}
	if err := cfg.Default(); err != nil {
		return nil, fmt.Errorf("error setting default values for kw_garden config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("error validating kw_garden config: %w", err)
	}
	return cfg, nil
}
