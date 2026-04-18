package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	configgen "github.com/Gabrielbdd/gofra/internal/generate/config"
	"github.com/Gabrielbdd/gofra/internal/scaffold"
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
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "gofra generate: %v\n", err)
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

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	if flags.NArg() != 1 {
		return fmt.Errorf("usage: gofra new [--module module/path] <directory>")
	}

	targetDir := flags.Arg(0)

	opts := scaffold.Options{
		Destination: targetDir,
		ModulePath:  *modulePath,
	}
	if err := scaffold.Generate(opts); err != nil {
		return err
	}

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		absTarget = targetDir
	}

	fmt.Fprintf(os.Stdout, "created %s\n\nnext steps:\n  cd %s\n  mise trust\n  mise run dev\n",
		absTarget, filepath.Base(absTarget))
	return nil
}

func runGenerate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: gofra generate <subcommand> [flags]")
	}

	switch args[0] {
	case "config":
		err := configgen.Run(args[1:], os.Stderr)
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	default:
		return fmt.Errorf("unknown generator %q", args[0])
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gofra new [--module module/path] <directory>")
	fmt.Fprintln(w, "  gofra generate config [flags] <proto-file>")
}
