package runtimeconfig_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	runtimeconfig "databit.com.br/gofra/runtime/config"

	flag "github.com/spf13/pflag"
)

// --- test types ---

type testConfig struct {
	App    testAppConfig `koanf:"app"`
	Public testPublic    `koanf:"public"`
}

type testAppConfig struct {
	Name string `koanf:"name"`
	Port int    `koanf:"port"`
}

type testPublic struct {
	AppName string   `koanf:"app_name"`
	Auth    testAuth `koanf:"auth"`
}

type testAuth struct {
	ClientID string   `koanf:"client_id"`
	Scopes   []string `koanf:"scopes"`
}

func testDefaults() testConfig {
	return testConfig{
		App: testAppConfig{Name: "default-app", Port: 3000},
		Public: testPublic{
			AppName: "default-app",
			Auth: testAuth{
				ClientID: "default-client",
				Scopes:   []string{"openid"},
			},
		},
	}
}

// --- validatable type ---

type validatableConfig struct {
	Port int `koanf:"port"`
}

func (c *validatableConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("port out of range")
	}
	return nil
}

// --- helpers ---

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gofra.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

// --- tests ---

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithoutYAML(),
		runtimeconfig.WithoutEnv(),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "default-app" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "default-app")
	}
	if cfg.App.Port != 3000 {
		t.Errorf("App.Port = %d, want %d", cfg.App.Port, 3000)
	}
	if cfg.Public.AppName != "default-app" {
		t.Errorf("Public.AppName = %q, want %q", cfg.Public.AppName, "default-app")
	}
}

func TestLoadYAML(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, `
app:
  name: from-yaml
  port: 4000
public:
  app_name: yaml-public
`)
	cfg, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithConfigPath(path),
		runtimeconfig.WithoutEnv(),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "from-yaml" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "from-yaml")
	}
	if cfg.App.Port != 4000 {
		t.Errorf("App.Port = %d, want %d", cfg.App.Port, 4000)
	}
	if cfg.Public.AppName != "yaml-public" {
		t.Errorf("Public.AppName = %q, want %q", cfg.Public.AppName, "yaml-public")
	}
	// Defaults preserved for fields not in YAML.
	if cfg.Public.Auth.ClientID != "default-client" {
		t.Errorf("Public.Auth.ClientID = %q, want %q", cfg.Public.Auth.ClientID, "default-client")
	}
}

func TestLoadEnvOverridesYAML(t *testing.T) {
	path := writeYAML(t, `
app:
  name: from-yaml
  port: 4000
`)
	setEnv(t, "GOFRA_APP__PORT", "5000")

	cfg, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithConfigPath(path),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "from-yaml" {
		t.Errorf("App.Name = %q, want %q (from YAML)", cfg.App.Name, "from-yaml")
	}
	if cfg.App.Port != 5000 {
		t.Errorf("App.Port = %d, want %d (from env)", cfg.App.Port, 5000)
	}
}

func TestLoadFlagsOverrideEnv(t *testing.T) {
	path := writeYAML(t, `
app:
  port: 4000
`)
	setEnv(t, "GOFRA_APP__PORT", "5000")

	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.Int("app.port", 0, "HTTP port")

	cfg, err := runtimeconfig.Load(testDefaults(), []string{"--app.port=6000"},
		runtimeconfig.WithConfigPath(path),
		runtimeconfig.WithFlags(flags),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Port != 6000 {
		t.Errorf("App.Port = %d, want %d (from flags)", cfg.App.Port, 6000)
	}
}

func TestLoadFullPrecedence(t *testing.T) {
	path := writeYAML(t, `
app:
  name: from-yaml
  port: 4000
public:
  app_name: yaml-public
`)
	setEnv(t, "GOFRA_APP__PORT", "5000")
	setEnv(t, "GOFRA_PUBLIC__APP_NAME", "env-public")

	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.Int("app.port", 0, "HTTP port")
	flags.String("app.name", "", "app name")

	cfg, err := runtimeconfig.Load(testDefaults(), []string{"--app.port=6000"},
		runtimeconfig.WithConfigPath(path),
		runtimeconfig.WithFlags(flags),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Flag overrides env and YAML.
	if cfg.App.Port != 6000 {
		t.Errorf("App.Port = %d, want %d (flag)", cfg.App.Port, 6000)
	}
	// YAML value preserved when flag not set.
	if cfg.App.Name != "from-yaml" {
		t.Errorf("App.Name = %q, want %q (YAML, flag not set)", cfg.App.Name, "from-yaml")
	}
	// Env overrides YAML.
	if cfg.Public.AppName != "env-public" {
		t.Errorf("Public.AppName = %q, want %q (env)", cfg.Public.AppName, "env-public")
	}
	// Default preserved when nothing overrides.
	if cfg.Public.Auth.ClientID != "default-client" {
		t.Errorf("Public.Auth.ClientID = %q, want %q (default)", cfg.Public.Auth.ClientID, "default-client")
	}
}

func TestLoadMissingYAMLIsNotError(t *testing.T) {
	t.Parallel()

	cfg, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithConfigPath("/nonexistent/gofra.yaml"),
		runtimeconfig.WithoutEnv(),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Port != 3000 {
		t.Errorf("App.Port = %d, want %d (default)", cfg.App.Port, 3000)
	}
}

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, `{{{invalid yaml`)

	_, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithConfigPath(path),
		runtimeconfig.WithoutEnv(),
	)
	if err == nil {
		t.Fatal("Load() expected error for invalid YAML, got nil")
	}
}

func TestLoadEnvUnderscoreInKeyName(t *testing.T) {
	setEnv(t, "GOFRA_PUBLIC__APP_NAME", "env-app")
	setEnv(t, "GOFRA_PUBLIC__AUTH__CLIENT_ID", "env-client")

	cfg, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithoutYAML(),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Public.AppName != "env-app" {
		t.Errorf("Public.AppName = %q, want %q", cfg.Public.AppName, "env-app")
	}
	if cfg.Public.Auth.ClientID != "env-client" {
		t.Errorf("Public.Auth.ClientID = %q, want %q", cfg.Public.Auth.ClientID, "env-client")
	}
}

func TestLoadValidationPasses(t *testing.T) {
	t.Parallel()

	cfg, err := runtimeconfig.Load(validatableConfig{Port: 8080}, nil,
		runtimeconfig.WithoutYAML(),
		runtimeconfig.WithoutEnv(),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
}

func TestLoadValidationFails(t *testing.T) {
	t.Parallel()

	_, err := runtimeconfig.Load(validatableConfig{Port: 0}, nil,
		runtimeconfig.WithoutYAML(),
		runtimeconfig.WithoutEnv(),
	)
	if err == nil {
		t.Fatal("Load() expected validation error, got nil")
	}
}

func TestLoadConfigPathEnv(t *testing.T) {
	path := writeYAML(t, `
app:
  name: from-config-env
`)
	setEnv(t, "GOFRA_CONFIG", path)

	cfg, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithoutEnv(),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "from-config-env" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "from-config-env")
	}
}

func TestLoadCustomEnvPrefix(t *testing.T) {
	setEnv(t, "MYAPP_APP__PORT", "7000")

	cfg, err := runtimeconfig.Load(testDefaults(), nil,
		runtimeconfig.WithoutYAML(),
		runtimeconfig.WithEnvPrefix("MYAPP_"),
	)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Port != 7000 {
		t.Errorf("App.Port = %d, want %d", cfg.App.Port, 7000)
	}
}
