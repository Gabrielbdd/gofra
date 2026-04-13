package configgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateProducesAllFiles(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "config")

	err := Generate(Options{
		ProtoFile:     testdataPath("full.proto"),
		OutputDir:     outDir,
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	for _, name := range []string{
		"config_gen.go",
		"load_gen.go",
		"public_gen.go",
	} {
		path := filepath.Join(outDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("missing generated file %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("generated file %s is empty", name)
		}
	}
}

func TestGenerateMinimalProto(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "config")

	err := Generate(Options{
		ProtoFile:     testdataPath("minimal.proto"),
		OutputDir:     outDir,
		GoPackage:     "config",
		RuntimeImport: "example.com/framework/runtime/config",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "config_gen.go"))
	if err != nil {
		t.Fatalf("read config_gen.go: %v", err)
	}

	src := string(content)
	if len(src) == 0 {
		t.Fatal("config_gen.go is empty")
	}
}

func TestGenerateMissingProto(t *testing.T) {
	t.Parallel()

	err := Generate(Options{
		ProtoFile: "/nonexistent/config.proto",
		OutputDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for missing proto file")
	}
}
