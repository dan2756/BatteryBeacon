// Command batt prints battery stats as a summary, JSON, a single value or a
// custom template.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/dan2756/batt/internal/battery"
	"github.com/dan2756/batt/internal/render"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = ""

var usage = `batt prints battery stats.

Usage:
  batt                       one-line summary
  batt --json                all stats as JSON
  batt --format TEMPLATE     custom output, e.g. '{percent}% {state}'
  batt percent               charge level, e.g. 87
  batt state                 charging, discharging, charged or not charging
  batt time                  H:MM until empty (or until full while charging)
  batt health                capacity vs. design, e.g. 91
  batt watch [options]       run commands on low/high charge, plug, unplug
                             (see 'batt watch -h')

Options:
  --json                     print JSON instead of a summary
  --format TEMPLATE          print TEMPLATE with {field}s filled in
  -h, --help                 show this help
  --version                  print the version

Template fields:
` + wrapFields(render.Fields, "  ", 76) + `
  Values the platform doesn't report print as nothing. {{ and }} are
  literal braces.

batt exits with status 1 if the machine has no battery, 2 on bad usage.
`

// commands maps single-value subcommands to the template field they print.
var commands = map[string]string{
	"percent": "percent",
	"state":   "state",
	"time":    "time",
	"health":  "health",
}

// readBattery is replaced in tests.
var readBattery = battery.Read

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "watch" {
		return runWatch(args[1:], stdout, stderr)
	}

	fs := flag.NewFlagSet("batt", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "")
	format := fs.String("format", "", "")
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

	var tmpl *render.Template
	switch {
	case fs.NArg() > 1:
		return usageError(stderr, fmt.Sprintf("unexpected argument %q", fs.Arg(1)))
	case fs.NArg() == 1:
		field, ok := commands[fs.Arg(0)]
		if !ok {
			return usageError(stderr, fmt.Sprintf("unknown command %q", fs.Arg(0)))
		}
		if *jsonOut || *format != "" {
			return usageError(stderr, fmt.Sprintf("%s takes no options", fs.Arg(0)))
		}
		tmpl, _ = render.ParseTemplate("{" + field + "}")
	case *format != "":
		if *jsonOut {
			return usageError(stderr, "--json and --format can't be combined")
		}
		var err error
		if tmpl, err = render.ParseTemplate(*format); err != nil {
			return usageError(stderr, err.Error())
		}
	}

	info, err := readBattery()
	if err != nil {
		fmt.Fprintf(stderr, "batt: %v\n", err)
		return 1
	}
	switch {
	case tmpl != nil:
		fmt.Fprintln(stdout, tmpl.Execute(info))
	case *jsonOut:
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(info); err != nil {
			fmt.Fprintf(stderr, "batt: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintln(stdout, render.Summary(info))
	}
	return 0
}

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "batt: %s\nRun 'batt -h' for usage.\n", msg)
	return 2
}

// wrapFields lays names out as {name} {name} ... lines no wider than width.
func wrapFields(names []string, indent string, width int) string {
	var lines []string
	line := indent
	for _, n := range names {
		f := "{" + n + "}"
		if len(line) > len(indent) && len(line)+1+len(f) > width {
			lines = append(lines, line)
			line = indent
		}
		if len(line) > len(indent) {
			line += " "
		}
		line += f
	}
	return strings.Join(append(lines, line), "\n")
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
