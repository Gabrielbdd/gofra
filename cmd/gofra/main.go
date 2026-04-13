package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"databit.com.br/gofra/internal/projectgen"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "new":
		if err := runNew(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "gofra new: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		usage(os.Stderr)
		os.Exit(2)
	}
}

func runNew(args []string) error {
	flags := flag.NewFlagSet("new", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	modulePath := flags.String("module", "", "Go module path for the generated application")
	frameworkDir := flags.String("framework-dir", "", "path to the local gofra framework checkout")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() != 1 {
		return fmt.Errorf("usage: gofra new [--module module/path] [--framework-dir /path/to/gofra] <directory>")
	}

	targetDir := flags.Arg(0)

	framework, err := resolveFramework(*frameworkDir)
	if err != nil {
		return err
	}

	opts := projectgen.Options{
		Destination:     targetDir,
		ModulePath:      *modulePath,
		FrameworkDir:    framework.Dir,
		FrameworkModule: framework.Module,
	}
	if err := projectgen.Generate(opts); err != nil {
		return err
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		absTarget = targetDir
	}

	fmt.Fprintf(os.Stdout, "created %s\n", absTarget)
	return nil
}

func resolveFramework(frameworkDir string) (projectgen.Framework, error) {
	if frameworkDir != "" {
		return projectgen.LoadFramework(frameworkDir)
	}

	wd, err := os.Getwd()
	if err != nil {
		return projectgen.Framework{}, err
	}

	return projectgen.DetectFramework(wd)
}

func usage(w *os.File) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gofra new [--module module/path] [--framework-dir /path/to/gofra] <directory>")
}
