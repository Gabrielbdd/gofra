package runtimeconfig

import (
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Load reads configuration into a typed struct using four layers of
// precedence (lowest to highest): struct defaults, YAML file, environment
// variables, and CLI flags.
//
// The defaults parameter supplies the base values (typically from a Default()
// function). Struct fields must carry koanf:"..." tags for correct key
// mapping.
//
// Environment variables use a double-underscore (__) convention for nesting:
//
//	GOFRA_APP__PORT=4000          -> app.port
//	GOFRA_PUBLIC__APP_NAME=MyApp  -> public.app_name
//
// If *T implements Validate() error, it is called after unmarshalling.
func Load[T any](defaults T, args []string, opts ...LoadOption) (*T, error) {
	s := defaultLoadSettings()
	for _, opt := range opts {
		if opt != nil {
			opt(&s)
		}
	}

	k := koanf.New(s.delimiter)

	// Layer 1: Defaults from struct.
	if err := k.Load(structs.Provider(defaults, "koanf"), nil); err != nil {
		return nil, fmt.Errorf("runtimeconfig: load defaults: %w", err)
	}

	// Layer 2: YAML file.
	if !s.skipYAML {
		configPath := s.configPath
		if p := os.Getenv(s.configPathEnv); p != "" {
			configPath = p
		}
		if _, err := os.Stat(configPath); err == nil {
			if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("runtimeconfig: load %s: %w", configPath, err)
			}
		}
	}

	// Layer 3: Environment variables.
	// Double-underscore (__) separates nesting levels; single underscores are
	// preserved as literal characters within a key segment.
	if !s.skipEnv {
		if err := k.Load(env.Provider(s.envPrefix, s.delimiter, func(key string) string {
			return strings.ReplaceAll(
				strings.ToLower(strings.TrimPrefix(key, s.envPrefix)),
				"__", s.delimiter,
			)
		}), nil); err != nil {
			return nil, fmt.Errorf("runtimeconfig: load env: %w", err)
		}
	}

	// Layer 4: CLI flags (only explicitly-set flags override).
	if !s.skipFlags && s.flags != nil {
		if err := s.flags.Parse(args); err != nil {
			return nil, fmt.Errorf("runtimeconfig: parse flags: %w", err)
		}
		if err := k.Load(posflag.Provider(s.flags, s.delimiter, k), nil); err != nil {
			return nil, fmt.Errorf("runtimeconfig: load flags: %w", err)
		}
	}

	// Unmarshal into typed struct.
	var cfg T
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{
		Tag: "koanf",
	}); err != nil {
		return nil, fmt.Errorf("runtimeconfig: unmarshal: %w", err)
	}

	// Validation: if *T implements Validate() error, call it.
	if v, ok := any(&cfg).(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return nil, fmt.Errorf("runtimeconfig: validate: %w", err)
		}
	}

	return &cfg, nil
}
