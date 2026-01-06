// Command batt prints battery stats as a one-line summary or JSON.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/dan2756/batt/internal/battery"
	"github.com/dan2756/batt/internal/render"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = ""

const usage = `batt prints battery stats.

Usage:
  batt              one-line summary
  batt --json       all stats as JSON

Options:
  --json            print JSON instead of a summary
  -h, --help        show this help
  --version         print the version

batt exits with status 1 if the machine has no battery.
`

// readBattery is replaced in tests.
var readBattery = battery.Read

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("batt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "")
	showVersion := fs.Bool("version", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		return usageError(stderr, err.Error())
	}
	if *showVersion {
		fmt.Fprintln(stdout, "batt", versionString())
		return 0
	}
	if fs.NArg() > 0 {
		return usageError(stderr, fmt.Sprintf("unexpected argument %q", fs.Arg(0)))
	}

	info, err := readBattery()
	if err != nil {
		fmt.Fprintf(stderr, "batt: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			fmt.Fprintf(stderr, "batt: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, render.Summary(info))
	return 0
}

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "batt: %s\nRun 'batt -h' for usage.\n", msg)
	return 2
}

func versionString() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
