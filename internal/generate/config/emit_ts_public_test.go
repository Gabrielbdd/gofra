package configgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmitTSPublic_FullProtoContainsPublicInterfaces(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("full.proto"), ParseOptions{
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
	})
	if err != nil {
		t.Fatalf("ParseProto: %v", err)
	}

	content, err := EmitTSPublic(schema, "__GOFRA_CONFIG__")
	if err != nil {
		t.Fatalf("EmitTSPublic: %v", err)
	}
	if content == nil {
		t.Fatal("EmitTSPublic returned nil content for proto with public subtree")
	}

	src := string(content)
	mustContain := []string{
		"export interface PublicAuthConfig {",
		"export interface PublicConfig {",
		"appName: string;",
		"apiBaseUrl: string;",
		"scopes: string[];",
		"auth: PublicAuthConfig;",
		"export type RuntimeConfig = PublicConfig;",
		"__GOFRA_CONFIG__?: RuntimeConfig;",
		"export const runtimeConfig: Partial<RuntimeConfig>",
		"export function loadRuntimeConfig(): RuntimeConfig",
	}
	for _, want := range mustContain {
		if !strings.Contains(src, want) {
			t.Errorf("emitted TS missing substring %q\nfull output:\n%s", want, src)
		}
	}
}

func TestEmitTSPublic_RespectsDependencyOrdering(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("full.proto"), ParseOptions{GoPackage: "config"})
	if err != nil {
		t.Fatalf("ParseProto: %v", err)
	}

	content, err := EmitTSPublic(schema, "")
	if err != nil {
		t.Fatalf("EmitTSPublic: %v", err)
	}

	src := string(content)
	authIdx := strings.Index(src, "export interface PublicAuthConfig")
	publicIdx := strings.Index(src, "export interface PublicConfig")
	if authIdx < 0 || publicIdx < 0 {
		t.Fatalf("expected both interfaces; authIdx=%d publicIdx=%d", authIdx, publicIdx)
	}
	if authIdx > publicIdx {
		t.Errorf("PublicAuthConfig should come before PublicConfig (dependency order); authIdx=%d publicIdx=%d", authIdx, publicIdx)
	}
}

func TestEmitTSPublic_CustomGlobalName(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("full.proto"), ParseOptions{GoPackage: "config"})
	if err != nil {
		t.Fatalf("ParseProto: %v", err)
	}

	content, err := EmitTSPublic(schema, "__MYAPP_CONFIG__")
	if err != nil {
		t.Fatalf("EmitTSPublic: %v", err)
	}

	src := string(content)
	if !strings.Contains(src, "__MYAPP_CONFIG__?: RuntimeConfig;") {
		t.Errorf("emitted TS should use custom global name __MYAPP_CONFIG__")
	}
	if !strings.Contains(src, "window.__MYAPP_CONFIG__") {
		t.Errorf("loader should read window.__MYAPP_CONFIG__")
	}
}

func TestEmitTSPublic_MinimalProtoNoNestedMessages(t *testing.T) {
	t.Parallel()

	schema, err := ParseProto(testdataPath("minimal.proto"), ParseOptions{GoPackage: "config"})
	if err != nil {
		t.Fatalf("ParseProto: %v", err)
	}

	content, err := EmitTSPublic(schema, "")
	if err != nil {
		t.Fatalf("EmitTSPublic: %v", err)
	}
	if content == nil {
		t.Fatal("minimal.proto has a public subtree; expected content")
	}

	src := string(content)
	if !strings.Contains(src, "export interface PublicConfig {") {
		t.Errorf("expected PublicConfig interface in minimal emission")
	}
	if !strings.Contains(src, "appName: string;") {
		t.Errorf("expected appName field")
	}
}

func TestEmitTSPublic_NilSchemaReturnsNil(t *testing.T) {
	t.Parallel()

	content, err := EmitTSPublic(nil, "")
	if err != nil {
		t.Fatalf("EmitTSPublic(nil): %v", err)
	}
	if content != nil {
		t.Errorf("expected nil content for nil schema; got %d bytes", len(content))
	}
}

func TestGenerate_WritesTSWhenTSOutDirSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	tsDir := filepath.Join(root, "web", "src", "gen")

	err := Generate(Options{
		ProtoFile:     testdataPath("full.proto"),
		OutputDir:     filepath.Join(root, "config"),
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
		TSOutDir:      tsDir,
		TSGlobalName:  "__GOFRA_CONFIG__",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	info, err := os.Stat(filepath.Join(tsDir, "runtime-config.ts"))
	if err != nil {
		t.Fatalf("expected runtime-config.ts to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("runtime-config.ts is empty")
	}
}

func TestGenerate_SkipsTSWhenTSOutDirEmpty(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	err := Generate(Options{
		ProtoFile:     testdataPath("full.proto"),
		OutputDir:     filepath.Join(root, "config"),
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "web")); !os.IsNotExist(err) {
		t.Errorf("expected no web/ dir when TSOutDir is empty; err=%v", err)
	}
}
