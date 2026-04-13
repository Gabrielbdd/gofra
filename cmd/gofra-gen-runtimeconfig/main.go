package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"databit.com.br/gofra/internal/runtimeconfiggen"
)

func main() {
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

	flag.StringVar(&goOut, "go-out", "", "write the scaffolded Go binder to this path")
	flag.StringVar(&goPackage, "go-package", "config", "package name for the scaffolded Go binder")
	flag.StringVar(&goConfigType, "go-config-type", "Config", "config type name for the scaffolded Go binder")
	flag.StringVar(&goFunction, "go-function", "BindPublicConfig", "function name for the scaffolded Go binder")
	flag.StringVar(&runtimeImport, "runtime-import", "", "import path for the runtime config Go package")
	flag.StringVar(&tsOut, "ts-out", "", "write the scaffolded TS loader to this path")
	flag.StringVar(&tsImport, "ts-import", "./runtime_config_pb", "import path for the runtime config TS types")
	flag.StringVar(&globalName, "global-name", "__GOFRA_CONFIG__", "browser global used by the runtime config script")
	flag.Parse()

	if goOut == "" && tsOut == "" {
		fmt.Fprintln(os.Stderr, "gofra-gen-runtimeconfig: at least one of -go-out or -ts-out is required")
		os.Exit(2)
	}

	if goOut != "" {
		content, err := runtimeconfiggen.RenderGoBinderStub(runtimeconfiggen.GoBinderParams{
			PackageName:   goPackage,
			RuntimeImport: runtimeImport,
			ConfigType:    goConfigType,
			FunctionName:  goFunction,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "gofra-gen-runtimeconfig: render go binder: %v\n", err)
			os.Exit(1)
		}
		if err := writeFile(goOut, content); err != nil {
			fmt.Fprintf(os.Stderr, "gofra-gen-runtimeconfig: write go binder: %v\n", err)
			os.Exit(1)
		}
	}

	if tsOut != "" {
		content, err := runtimeconfiggen.RenderTSLoader(runtimeconfiggen.TSLoaderParams{
			RuntimeImport: tsImport,
			GlobalName:    globalName,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "gofra-gen-runtimeconfig: render ts loader: %v\n", err)
			os.Exit(1)
		}
		if err := writeFile(tsOut, content); err != nil {
			fmt.Fprintf(os.Stderr, "gofra-gen-runtimeconfig: write ts loader: %v\n", err)
			os.Exit(1)
		}
	}
}

func writeFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
