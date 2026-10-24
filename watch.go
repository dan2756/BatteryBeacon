package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/dan2756/batt/internal/battery"
	"github.com/dan2756/batt/internal/watch"
)

const watchUsage = `Usage: batt watch [options]

Polls the battery and runs shell commands when the charge crosses a
threshold, the charger is plugged in or out, or the battery is full.

Options:
  --below N CMD        run CMD when the charge drops to N% or lower (repeatable)
  --above N CMD        run CMD when the charge rises to N% or higher (repeatable)
  --on-plug CMD        run CMD when the charger is connected
  --on-unplug CMD      run CMD when the charger is disconnected
  --on-full CMD        run CMD when the battery is fully charged
  -i, --interval DUR   time between readings, e.g. 30s or 5m (default 60s)
  -v, --verbose        log readings and hooks to stderr
  -h, --help           show this help

Each threshold fires once per crossing and re-arms after the charge moves 2
points back (to 22% for --below 20). Within 2 points of 0% or 100% it
re-arms at that end of the scale instead (--below 99 re-arms at 100%).
Thresholds and --on-full also fire on the first reading if they already hold.

Hooks run with /bin/sh -c and get these environment variables:
  BATT_EVENT         below, above, plug, unplug or full
  BATT_PERCENT       charge level, e.g. 19
  BATT_STATE         charging, discharging, charged or not charging
  BATT_POWER_SOURCE  ac or battery
  BATT_THRESHOLD     the threshold that fired (below/above only)

Example:
  batt watch --below 20 'notify "battery low: $BATT_PERCENT%"' \
             --above 80 'notify "unplug"' -i 60s
`

type watchConfig struct {
	hooks    watch.Hooks
	interval time.Duration
	verbose  bool
}

func runWatch(args []string, stdout, stderr io.Writer) int {
	cfg, err := parseWatchArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		_, err = fmt.Fprint(stdout, watchUsage)
		return checkWrite(stderr, err)
	}
	if err != nil {
		fmt.Fprintf(stderr, "batt watch: %v\nRun 'batt watch -h' for usage.\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), stopSignals()...)
	defer stop()
	if err := watchLoop(ctx, cfg, readBattery, shellHook(stdout, stderr), stderr); err != nil {
		fmt.Fprintf(stderr, "batt: %v\n", err)
		return 1
	}
	return 0
}

// parseWatchArgs parses watch's arguments by hand, since --below and
// --above take two values each.
func parseWatchArgs(args []string) (watchConfig, error) {
	cfg := watchConfig{interval: 60 * time.Second}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" || arg == "--" {
			return cfg, fmt.Errorf("unexpected argument %q", arg)
		}
		name, inline, hasInline := strings.Cut(arg, "=")
		key := strings.TrimPrefix(strings.TrimPrefix(name, "-"), "-")

		// value returns the flag's next value: the part after "=" first,
		// then the following arguments.
		value := func(what string) (string, error) {
			if hasInline {
				hasInline = false
				return inline, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs %s", name, what)
			}
			i++
			return args[i], nil
		}

		switch key {
		case "below", "above":
			lvl, err := value("a percentage and a command")
			if err != nil {
				return cfg, err
			}
			level, err := parseLevel(lvl)
			if err != nil {
				return cfg, fmt.Errorf("%s: %v", name, err)
			}
			cmd, err := value("a command after the percentage")
			if err != nil {
				return cfg, err
			}
			cfg.hooks.Thresholds = append(cfg.hooks.Thresholds, watch.Threshold{
				Above: key == "above", Level: level, Command: cmd,
			})
		case "on-plug", "on-unplug", "on-full":
			if seen[key] {
				return cfg, fmt.Errorf("--%s given more than once", key)
			}
			seen[key] = true
			cmd, err := value("a command")
			if err != nil {
				return cfg, err
			}
			switch key {
			case "on-plug":
				cfg.hooks.OnPlug = cmd
			case "on-unplug":
				cfg.hooks.OnUnplug = cmd
			case "on-full":
				cfg.hooks.OnFull = cmd
			}
		case "i", "interval":
			v, err := value("a duration")
			if err != nil {
				return cfg, err
			}
			if cfg.interval, err = parseInterval(v); err != nil {
				return cfg, fmt.Errorf("%s: %v", name, err)
			}
		case "v", "verbose":
			cfg.verbose = true
		case "h", "help":
			return cfg, flag.ErrHelp
		default:
			return cfg, fmt.Errorf("unknown option %s", name)
		}
		if hasInline {
			return cfg, fmt.Errorf("%s doesn't take a value", name)
		}
	}
	if cfg.hooks.Empty() {
		return cfg, errors.New("nothing to do: give at least one of --below, --above, --on-plug, --on-unplug or --on-full")
	}
	return cfg, nil
}

func parseLevel(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSuffix(s, "%"))
	if err != nil || n < 0 || n > 100 {
		return 0, fmt.Errorf("%q is not a percentage between 0 and 100", s)
	}
	return n, nil
}

// parseInterval accepts Go durations ("30s", "5m") and bare seconds ("60").
func parseInterval(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		n, nerr := strconv.Atoi(s)
		if nerr != nil {
			return 0, fmt.Errorf("%q is not a duration like 30s or 5m", s)
		}
		d = time.Duration(n) * time.Second
	}
	if d < time.Second {
		return 0, fmt.Errorf("%s is too short; use at least 1s", d)
	}
	return d, nil
}

type hookFunc func(ctx context.Context, ev watch.Event) error

// watchLoop reads the battery every cfg.interval until ctx is done, running
// hooks as the watcher asks for them. A failure on the first reading (such
// as no battery) is returned; later failures are logged and retried.
func watchLoop(ctx context.Context, cfg watchConfig, read func() (*battery.Info, error), hook hookFunc, logw io.Writer) error {
	w := watch.New(cfg.hooks)
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()
	for first := true; ; first = false {
		info, err := read()
		switch {
		case err != nil && first:
			return err
		case err != nil:
			fmt.Fprintf(logw, "batt: %v\n", err)
		default:
			r := watch.ReadingOf(info)
			if cfg.verbose {
				fmt.Fprintf(logw, "batt: %d%% %s on %s\n", r.Percent, r.State, r.Source)
			}
			for _, ev := range w.Observe(r) {
				if cfg.verbose {
					fmt.Fprintf(logw, "batt: %s: running %s\n", ev, ev.Command)
				}
				if err := hook(ctx, ev); err != nil && ctx.Err() == nil {
					fmt.Fprintf(logw, "batt: %s hook: %v\n", ev, err)
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if ctx.Err() != nil {
				return nil // both were ready; stopping wins
			}
		}
	}
}

// shellHook runs hooks with /bin/sh, one at a time, with their output going
// to stdout and stderr. A hook still running when ctx is done is stopped
// along with everything it started.
func shellHook(stdout, stderr io.Writer) hookFunc {
	return func(ctx context.Context, ev watch.Event) error {
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", ev.Command)
		cmd.Env = append(os.Environ(), hookEnv(ev)...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return runHook(cmd)
	}
}

func hookEnv(ev watch.Event) []string {
	env := []string{
		"BATT_EVENT=" + ev.Name,
		"BATT_PERCENT=" + strconv.Itoa(ev.Reading.Percent),
		"BATT_STATE=" + string(ev.Reading.State),
		"BATT_POWER_SOURCE=" + string(ev.Reading.Source),
	}
	if ev.Name == watch.Below || ev.Name == watch.Above {
		env = append(env, "BATT_THRESHOLD="+strconv.Itoa(ev.Threshold))
	}
	return env
}
