package scaffold

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateCreatesRunnableStarter(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	framework, err := DetectFramework(wd)
	if err != nil {
		t.Fatalf("DetectFramework() error = %v", err)
	}

	destination := filepath.Join(t.TempDir(), "myapp")
	if err := Generate(Options{
		Destination:     destination,
		ModulePath:      "example.com/myapp",
		FrameworkDir:    framework.Dir,
		FrameworkModule: framework.Module,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	for _, rel := range []string{
		"go.mod",
		"go.sum",
		"README.md",
		"cmd/app/main.go",
		"config/config_gen.go",
		"config/load_gen.go",
		"config/public_gen.go",
		"proto/myapp/config/v1/config.proto",
		"web/embed.go",
		"web/index.html",
	} {
		if _, err := os.Stat(filepath.Join(destination, rel)); err != nil {
			t.Fatalf("missing generated file %q: %v", rel, err)
		}
	}

	assertNoTokensRemain(t, destination)

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = destination
	cmd.Env = append(os.Environ(), "GOWORK=off")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test ./... failed: %v\n%s", err, output)
	}
}

func assertNoTokensRemain(t *testing.T, root string) {
	t.Helper()

	tokens := [][]byte{
		[]byte("__GOFRA_APP_NAME__"),
		[]byte("__GOFRA_MODULE__"),
		[]byte("__GOFRA_PROTO_PACKAGE__"),
		[]byte("__GOFRA_FRAMEWORK_DIR__"),
		[]byte("__GOFRA_FRAMEWORK_MODULE__"),
		[]byte("__GOFRA_FRAMEWORK_VERSION__"),
	}

	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, token := range tokens {
			if bytes.Contains(content, token) {
				t.Fatalf("generated file %q still contains token %q", path, string(token))
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
}
