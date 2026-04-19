package configgen

import (
	"flag"
	"fmt"
	"io"
)

// Run is the CLI entry point for `gofra generate config`.
func Run(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("config", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var (
		outDir        string
		goPackage     string
		runtimeImport string
		tsOutDir      string
		tsGlobalName  string
	)

	flags.StringVar(&outDir, "out", "config", "output directory for generated Go files")
	flags.StringVar(&goPackage, "package", "config", "Go package name for generated code")
	flags.StringVar(&runtimeImport, "runtime", "", "import path for the framework runtime/config package")
	flags.StringVar(&tsOutDir, "ts-out", "", "output directory for generated TypeScript (empty disables TS emission)")
	flags.StringVar(&tsGlobalName, "ts-global-name", "__GOFRA_CONFIG__", "window global name the TS loader reads the public config from")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() != 1 {
		return fmt.Errorf("usage: gofra generate config [flags] <proto-file>")
	}

	protoFile := flags.Arg(0)

	return Generate(Options{
		ProtoFile:     protoFile,
		OutputDir:     outDir,
		GoPackage:     goPackage,
		RuntimeImport: runtimeImport,
		TSOutDir:      tsOutDir,
		TSGlobalName:  tsGlobalName,
	})
}
