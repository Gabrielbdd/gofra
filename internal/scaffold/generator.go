package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	starterRoot             = "starter/full"
	defaultFrameworkVersion = "v0.0.0"
)

var (
	//go:embed all:starter/full
	starterFS embed.FS

	errDestinationRequired     = errors.New("destination is required")
	errFrameworkDirRequired    = errors.New("framework directory is required")
	errFrameworkModuleRequired = errors.New("framework module is required")
)

type Framework struct {
	Dir    string
	Module string
}

type Options struct {
	Destination     string
	ModulePath      string
	AppName         string
	ProtoPackage    string
	FrameworkDir    string
	FrameworkModule string
}

func DetectFramework(startDir string) (Framework, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return Framework{}, err
	}

	for {
		framework, err := LoadFramework(dir)
		if err == nil {
			return framework, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return Framework{}, fmt.Errorf("could not find gofra framework root from %q", startDir)
}

func LoadFramework(dir string) (Framework, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return Framework{}, err
	}

	if _, err := os.Stat(filepath.Join(absDir, "internal", "scaffold", "starter", "full")); err != nil {
		return Framework{}, fmt.Errorf("framework root %q does not contain internal/scaffold/starter/full", absDir)
	}

	modulePath, err := readModulePath(filepath.Join(absDir, "go.mod"))
	if err != nil {
		return Framework{}, err
	}

	return Framework{
		Dir:    absDir,
		Module: modulePath,
	}, nil
}

func Generate(opts Options) error {
	opts, err := fillDefaults(opts)
	if err != nil {
		return err
	}

	if err := ensureEmptyDestination(opts.Destination); err != nil {
		return err
	}

	replacements := map[string]string{
		"__GOFRA_APP_NAME__":          opts.AppName,
		"__GOFRA_MODULE__":            opts.ModulePath,
		"__GOFRA_PROTO_PACKAGE__":     opts.ProtoPackage,
		"__GOFRA_FRAMEWORK_DIR__":     filepath.ToSlash(opts.FrameworkDir),
		"__GOFRA_FRAMEWORK_MODULE__":  opts.FrameworkModule,
		"__GOFRA_FRAMEWORK_VERSION__": defaultFrameworkVersion,
	}

	root, err := fs.Sub(starterFS, starterRoot)
	if err != nil {
		return err
	}

	if err := fs.WalkDir(root, ".", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}

		renderedName := replaceTokens(name, replacements)
		if strings.HasSuffix(renderedName, ".tmpl") {
			renderedName = strings.TrimSuffix(renderedName, ".tmpl")
		}
		targetPath := filepath.Join(opts.Destination, filepath.FromSlash(renderedName))

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}

		content, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		renderedContent := []byte(replaceTokens(string(content), replacements))
		return os.WriteFile(targetPath, renderedContent, 0o644)
	}); err != nil {
		return err
	}

	return nil
}

func fillDefaults(opts Options) (Options, error) {
	if opts.Destination == "" {
		return Options{}, errDestinationRequired
	}
	if opts.FrameworkDir == "" {
		return Options{}, errFrameworkDirRequired
	}
	if opts.FrameworkModule == "" {
		return Options{}, errFrameworkModuleRequired
	}

	absDestination, err := filepath.Abs(opts.Destination)
	if err != nil {
		return Options{}, err
	}
	opts.Destination = absDestination

	if opts.AppName == "" {
		opts.AppName = filepath.Base(absDestination)
	}
	if opts.ModulePath == "" {
		opts.ModulePath = opts.AppName
	}
	if opts.ProtoPackage == "" {
		opts.ProtoPackage = normalizeProtoPackage(opts.AppName)
	}

	opts.FrameworkDir, err = filepath.Abs(opts.FrameworkDir)
	if err != nil {
		return Options{}, err
	}

	return opts, nil
}

func ensureEmptyDestination(destination string) error {
	info, err := os.Stat(destination)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return os.MkdirAll(destination, 0o755)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("destination %q is not a directory", destination)
	}

	entries, err := os.ReadDir(destination)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("destination %q is not empty", destination)
	}

	return nil
}

func normalizeProtoPackage(value string) string {
	var b strings.Builder
	lastUnderscore := false

	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		case !lastUnderscore:
			b.WriteRune('_')
			lastUnderscore = true
		}
	}

	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "app"
	}
	if result[0] >= '0' && result[0] <= '9' {
		return "app_" + result
	}

	return result
}

func readModulePath(goModPath string) (string, error) {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			if modulePath == "" {
				return "", fmt.Errorf("go.mod at %q has an empty module path", goModPath)
			}
			return modulePath, nil
		}
	}

	return "", fmt.Errorf("go.mod at %q does not declare a module path", goModPath)
}

func replaceTokens(value string, replacements map[string]string) string {
	for token, replacement := range replacements {
		value = strings.ReplaceAll(value, token, replacement)
	}
	return value
}

