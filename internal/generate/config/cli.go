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
	)

	flags.StringVar(&outDir, "out", "config", "output directory for generated Go files")
	flags.StringVar(&goPackage, "package", "config", "Go package name for generated code")
	flags.StringVar(&runtimeImport, "runtime", "", "import path for the framework runtime/config package")

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
	})
}
