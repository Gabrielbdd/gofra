package runtimeconfiggen

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var errOutputRequired = errors.New("at least one of -go-out or -ts-out is required")

func Run(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("runtime-config", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		goOut         string
		goPackage     string
		goConfigType  string
		goFunction    string
		runtimeImport string
		tsOut         string
		tsImport      string
		globalName    string
	)

	flags.StringVar(&goOut, "go-out", "", "write the scaffolded Go binder to this path")
	flags.StringVar(&goPackage, "go-package", "config", "package name for the scaffolded Go binder")
	flags.StringVar(&goConfigType, "go-config-type", "Config", "config type name for the scaffolded Go binder")
	flags.StringVar(&goFunction, "go-function", "BindPublicConfig", "function name for the scaffolded Go binder")
	flags.StringVar(&runtimeImport, "runtime-import", "", "import path for the runtime config Go package")
	flags.StringVar(&tsOut, "ts-out", "", "write the scaffolded TS loader to this path")
	flags.StringVar(&tsImport, "ts-import", "./runtime_config_pb", "import path for the runtime config TS types")
	flags.StringVar(&globalName, "global-name", "__GOFRA_CONFIG__", "browser global used by the runtime config script")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if goOut == "" && tsOut == "" {
		return errOutputRequired
	}

	if goOut != "" {
		content, err := RenderGoBinderStub(GoBinderParams{
			PackageName:   goPackage,
			RuntimeImport: runtimeImport,
			ConfigType:    goConfigType,
			FunctionName:  goFunction,
		})
		if err != nil {
			return fmt.Errorf("render go binder: %w", err)
		}
		if err := writeFile(goOut, content); err != nil {
			return fmt.Errorf("write go binder: %w", err)
		}
	}

	if tsOut != "" {
		content, err := RenderTSLoader(TSLoaderParams{
			RuntimeImport: tsImport,
			GlobalName:    globalName,
		})
		if err != nil {
			return fmt.Errorf("render ts loader: %w", err)
		}
		if err := writeFile(tsOut, content); err != nil {
			return fmt.Errorf("write ts loader: %w", err)
		}
	}

	return nil
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
