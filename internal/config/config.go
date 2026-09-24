// Package config applies settings from .acurl.yaml files and ACURL_*
// environment variables to command flags.
//
// Precedence, highest first: command-line flags, environment variables,
// the config file, flag defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// FileName is the per-project config file name.
const FileName = ".acurl.yaml"

// Config maps flag names to values, e.g. {"timeout": "10s", "agents": ["GPTBot"]}.
type Config map[string]any

// Find returns the config file to use: explicit if set, else ./.acurl.yaml,
// else <user config dir>/acurl/config.yaml. It returns "" when none exists.
func Find(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("config: %w", err)
		}
		return explicit, nil
	}
	candidates := []string{FileName}
	if dir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(dir, "acurl", "config.yaml"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("config: %w", err)
		}
	}
	return "", nil
}

// Load reads a config file. An empty path returns an empty Config.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	if c == nil {
		c = Config{}
	}
	return c, nil
}

// EnvName returns the environment variable for a flag: --cloak-threshold
// becomes ACURL_CLOAK_THRESHOLD.
func EnvName(flag string) string {
	return "ACURL_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
}

// Apply sets every flag in fs that was not given on the command line from
// the environment or, failing that, from c. Unknown config keys are
// returned as an error so typos do not go unnoticed.
func Apply(fs *pflag.FlagSet, c Config, getenv func(string) string) error {
	var errs []string
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			return
		}
		if v := getenv(EnvName(f.Name)); v != "" {
			if err := set(fs, f, v); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", EnvName(f.Name), err))
			}
			return
		}
		if v, ok := c[f.Name]; ok {
			if err := set(fs, f, v); err != nil {
				errs = append(errs, fmt.Sprintf("config %q: %v", f.Name, err))
			}
		}
	})
	if len(errs) > 0 {
		sort.Strings(errs)
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Unknown returns config keys that match no flag of any command.
func Unknown(c Config, known map[string]bool) []string {
	var out []string
	for k := range c {
		if !known[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func set(fs *pflag.FlagSet, f *pflag.Flag, v any) error {
	switch x := v.(type) {
	case []any:
		// Lists: comma-join for slice flags, repeated Set for array flags.
		if f.Value.Type() == "stringArray" {
			for _, item := range x {
				if err := fs.Set(f.Name, fmt.Sprint(item)); err != nil {
					return err
				}
			}
			return nil
		}
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = fmt.Sprint(item)
		}
		return fs.Set(f.Name, strings.Join(parts, ","))
	default:
		return fs.Set(f.Name, fmt.Sprint(x))
	}
}
