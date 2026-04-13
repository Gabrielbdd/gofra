package runtimeconfig

import (
	flag "github.com/spf13/pflag"
)

// LoadOption configures the behaviour of [Load].
type LoadOption func(*loadSettings)

type loadSettings struct {
	configPath    string
	configPathEnv string
	envPrefix     string
	delimiter     string
	flags         *flag.FlagSet
	skipYAML      bool
	skipEnv       bool
	skipFlags     bool
}

func defaultLoadSettings() loadSettings {
	return loadSettings{
		configPath:    "gofra.yaml",
		configPathEnv: "GOFRA_CONFIG",
		envPrefix:     "GOFRA_",
		delimiter:     ".",
	}
}

// WithConfigPath sets the default YAML config file path.
// The path can still be overridden by the GOFRA_CONFIG environment variable.
// Default: "gofra.yaml".
func WithConfigPath(path string) LoadOption {
	return func(s *loadSettings) { s.configPath = path }
}

// WithEnvPrefix sets the environment variable prefix used for config
// overrides. Double-underscore (__) separates nesting levels; single
// underscores are preserved as literal characters.
// Default: "GOFRA_".
func WithEnvPrefix(prefix string) LoadOption {
	return func(s *loadSettings) { s.envPrefix = prefix }
}

// WithFlags provides a pflag.FlagSet whose explicitly-set flags override
// environment variables. Flags that were not set on the command line are
// ignored. When WithFlags is not used the flags layer is skipped entirely.
func WithFlags(flags *flag.FlagSet) LoadOption {
	return func(s *loadSettings) { s.flags = flags }
}

// WithoutYAML disables YAML file loading.
func WithoutYAML() LoadOption {
	return func(s *loadSettings) { s.skipYAML = true }
}

// WithoutEnv disables environment variable loading.
func WithoutEnv() LoadOption {
	return func(s *loadSettings) { s.skipEnv = true }
}

// WithoutFlags disables CLI flag loading even when a FlagSet was provided.
func WithoutFlags() LoadOption {
	return func(s *loadSettings) { s.skipFlags = true }
}
