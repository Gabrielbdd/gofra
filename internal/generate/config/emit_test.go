package configgen

import (
	"strings"
	"testing"
)

func parseFullSchema(t *testing.T) *ConfigSchema {
	t.Helper()
	schema, err := ParseProto(testdataPath("full.proto"), ParseOptions{
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
	})
	if err != nil {
		t.Fatalf("ParseProto() error = %v", err)
	}
	return schema
}

func TestEmitConfigProducesValidGo(t *testing.T) {
	t.Parallel()
	schema := parseFullSchema(t)

	out, err := EmitConfig(schema)
	if err != nil {
		t.Fatalf("EmitConfig() error = %v", err)
	}

	src := string(out)

	// Check struct declarations.
	if !strings.Contains(src, "type Config struct") {
		t.Error("missing Config struct")
	}
	if !strings.Contains(src, "type AppConfig struct") {
		t.Error("missing AppConfig struct")
	}
	if !strings.Contains(src, "type PublicConfig struct") {
		t.Error("missing PublicConfig struct")
	}
	if !strings.Contains(src, "type DatabaseConfig struct") {
		t.Error("missing DatabaseConfig struct")
	}

	// Check koanf/yaml tags on app fields.
	if !strings.Contains(src, `koanf:"port"`) {
		t.Error("missing koanf tag on port")
	}
	if !strings.Contains(src, `yaml:"port"`) {
		t.Error("missing yaml tag on port")
	}

	// Check json tags ONLY on public fields.
	if !strings.Contains(src, `json:"appName"`) {
		t.Error("missing json tag on public app_name")
	}

	// App fields should NOT have json tags.
	// port is under AppConfig (not public), so check no json:"port" nearby.
	lines := strings.Split(src, "\n")
	for _, line := range lines {
		if strings.Contains(line, `koanf:"port"`) && strings.Contains(line, `json:`) {
			t.Error("app port should not have json tag")
		}
	}

	// Check DefaultConfig.
	if !strings.Contains(src, "func DefaultConfig() *Config") {
		t.Error("missing DefaultConfig()")
	}
	if !strings.Contains(src, "Port: 3000") {
		t.Error("missing Port default 3000")
	}
	if !strings.Contains(src, `Name: "testapp"`) {
		t.Error("missing Name default")
	}
	if !strings.Contains(src, "MaxOpenConns: 25") {
		t.Error("missing MaxOpenConns default")
	}
}

func TestEmitLoadProducesValidGo(t *testing.T) {
	t.Parallel()
	schema := parseFullSchema(t)

	out, err := EmitLoad(schema)
	if err != nil {
		t.Fatalf("EmitLoad() error = %v", err)
	}

	src := string(out)

	// Check NewFlagSet.
	if !strings.Contains(src, "func NewFlagSet()") {
		t.Error("missing NewFlagSet()")
	}

	// Check flags are registered with dotted paths.
	if !strings.Contains(src, `"app.port"`) {
		t.Error("missing app.port flag")
	}
	if !strings.Contains(src, `"app.name"`) {
		t.Error("missing app.name flag")
	}
	if !strings.Contains(src, `"public.app_name"`) {
		t.Error("missing public.app_name flag")
	}

	// Secret fields should NOT appear as flags.
	if strings.Contains(src, `"app.database.dsn"`) {
		t.Error("secret field dsn should not be a flag")
	}

	// Repeated fields should NOT appear as flags.
	if strings.Contains(src, `"public.auth.scopes"`) {
		t.Error("repeated field scopes should not be a flag")
	}

	// Check Load function.
	if !strings.Contains(src, "func Load(") {
		t.Error("missing Load()")
	}
	if !strings.Contains(src, "runtimeconfig.Load") {
		t.Error("Load should call runtimeconfig.Load")
	}
}

func TestEmitPublicProducesValidGo(t *testing.T) {
	t.Parallel()
	schema := parseFullSchema(t)

	out, err := EmitPublic(schema)
	if err != nil {
		t.Fatalf("EmitPublic() error = %v", err)
	}

	src := string(out)

	// Check BindPublicConfig.
	if !strings.Contains(src, "func BindPublicConfig(") {
		t.Error("missing BindPublicConfig()")
	}
	if !strings.Contains(src, "cfg.Public") {
		t.Error("BindPublicConfig should return cfg.Public")
	}

	// Check PublicConfigHandler.
	if !strings.Contains(src, "func PublicConfigHandler(") {
		t.Error("missing PublicConfigHandler()")
	}
	if !strings.Contains(src, "runtimeconfig.Handler") {
		t.Error("PublicConfigHandler should use runtimeconfig.Handler")
	}
}

func TestEmitConfigMinimalProducesValidGo(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("minimal.proto"), ParseOptions{
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
	})
	if err != nil {
		t.Fatalf("ParseProto() error = %v", err)
	}

	out, err := EmitConfig(schema)
	if err != nil {
		t.Fatalf("EmitConfig() error = %v", err)
	}

	src := string(out)
	if !strings.Contains(src, "Port: 3000") {
		t.Error("missing Port default")
	}
}
