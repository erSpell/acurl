// Package profiles loads and filters the agent profiles acurl fetches with.
package profiles

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/tragent-io/acurl/pkg/types"
	"gopkg.in/yaml.v3"
)

//go:embed profiles.yaml
var defaultProfiles []byte

// File is the on-disk layout of a profiles YAML file.
type File struct {
	Profiles []types.Profile `yaml:"profiles"`
}

// Default returns the embedded default profiles.
func Default() ([]types.Profile, error) {
	return Parse(defaultProfiles)
}

// Load reads profiles from path, or the embedded defaults when path is empty.
func Load(path string) ([]types.Profile, error) {
	if path == "" {
		return Default()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profiles: %w", err)
	}
	return Parse(data)
}

// Parse decodes and validates a profiles YAML document.
func Parse(data []byte) ([]types.Profile, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}
	if len(f.Profiles) == 0 {
		return nil, fmt.Errorf("no profiles found")
	}
	seen := make(map[string]bool, len(f.Profiles))
	for i, p := range f.Profiles {
		if p.Name == "" {
			return nil, fmt.Errorf("profile #%d: name is required", i+1)
		}
		if p.UserAgent == "" {
			return nil, fmt.Errorf("profile %q: user_agent is required", p.Name)
		}
		key := strings.ToLower(p.Name)
		if seen[key] {
			return nil, fmt.Errorf("duplicate profile name %q", p.Name)
		}
		seen[key] = true
	}
	return f.Profiles, nil
}

// Filter returns the profiles whose names match one of names
// (case-insensitive). An empty names list returns all profiles.
func Filter(profiles []types.Profile, names []string) []types.Profile {
	if len(names) == 0 {
		return profiles
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[strings.ToLower(strings.TrimSpace(n))] = true
	}
	var out []types.Profile
	for _, p := range profiles {
		if want[strings.ToLower(p.Name)] {
			out = append(out, p)
		}
	}
	return out
}
