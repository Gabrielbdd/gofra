package configgen

import (
	"fmt"
	"os"
	"path/filepath"
)

// Options configures the config generator.
type Options struct {
	// ProtoFile is the path to the config.proto file.
	ProtoFile string

	// OutputDir is the directory for generated Go files (default: "config/").
	OutputDir string

	// GoPackage is the Go package name (default: "config").
	GoPackage string

	// RuntimeImport is the import path for the framework's runtime/config package.
	RuntimeImport string
}

// Generate parses a config.proto file and generates Go code.
func Generate(opts Options) error {
	if opts.OutputDir == "" {
		opts.OutputDir = "config"
	}
	if opts.GoPackage == "" {
		opts.GoPackage = "config"
	}

	schema, err := ParseProto(opts.ProtoFile, ParseOptions{
		GoPackage:     opts.GoPackage,
		RuntimeImport: opts.RuntimeImport,
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("configgen: mkdir %s: %w", opts.OutputDir, err)
	}

	type emitter struct {
		name string
		fn   func(*ConfigSchema) ([]byte, error)
	}

	emitters := []emitter{
		{"config_gen.go", EmitConfig},
		{"load_gen.go", EmitLoad},
		{"public_gen.go", EmitPublic},
	}

	for _, e := range emitters {
		content, err := e.fn(schema)
		if err != nil {
			return fmt.Errorf("configgen: emit %s: %w", e.name, err)
		}
		path := filepath.Join(opts.OutputDir, e.name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("configgen: write %s: %w", path, err)
		}
	}

	return nil
}
