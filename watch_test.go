package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dan2756/batt/internal/battery"
	"github.com/dan2756/batt/internal/watch"
)

func TestParseWatchArgs(t *testing.T) {
	cfg, err := parseWatchArgs([]string{
		"--below", "20", `notify "battery low: $BATT_PERCENT%"`,
		"--below=10%", "shutdown-soon",
		"--above", "80", `notify "unplug"`,
		"--on-plug", "echo in",
		"--on-unplug=echo out",
		"-on-full", "echo full",
		"-i", "30s",
		"-v",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := watch.Hooks{
		Thresholds: []watch.Threshold{
			{Level: 20, Command: `notify "battery low: $BATT_PERCENT%"`},
			{Level: 10, Command: "shutdown-soon"},
			{Above: true, Level: 80, Command: `notify "unplug"`},
		},
		OnPlug:   "echo in",
		OnUnplug: "echo out",
		OnFull:   "echo full",
	}
	if !reflect.DeepEqual(cfg.hooks, want) {
		t.Errorf("hooks = %+v\nwant    %+v", cfg.hooks, want)
	}
	if cfg.interval != 30*time.Second || !cfg.verbose {
		t.Errorf("interval = %v, verbose = %v", cfg.interval, cfg.verbose)
	}
}

func TestParseWatchArgsDefaults(t *testing.T) {
	cfg, err := parseWatchArgs([]string{"--on-full", "x"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.interval != time.Minute || cfg.verbose {
		t.Errorf("interval = %v, verbose = %v", cfg.interval, cfg.verbose)
	}
	cfg, err = parseWatchArgs([]string{"--interval=90", "--on-full", "x"})
	if err != nil || cfg.interval != 90*time.Second {
		t.Errorf("bare seconds: interval = %v, err = %v", cfg.interval, err)
	}
}

func TestParseWatchArgsErrors(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, "nothing to do"},
		{[]string{"-i", "10s"}, "nothing to do"},
		{[]string{"--below"}, "needs a percentage"},
		{[]string{"--below", "20"}, "needs a command"},
		{[]string{"--below", "low", "cmd"}, "not a percentage"},
		{[]string{"--above", "120", "cmd"}, "not a percentage"},
		{[]string{"--on-plug"}, "needs a command"},
		{[]string{"--on-plug", "a", "--on-plug", "b"}, "more than once"},
		{[]string{"--on-full", "x", "-i", "100ms"}, "too short"},
		{[]string{"--on-full", "x", "-i", "soon"}, "not a duration"},
		{[]string{"--on-full", "x", "--verbose=yes"}, "doesn't take a value"},
		{[]string{"--on-full", "x", "--every", "5m"}, "unknown option --every"},
		{[]string{"--on-full", "x", "stray"}, `unexpected argument "stray"`},
	}
	for _, tt := range tests {
		_, err := parseWatchArgs(tt.args)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: err = %v, want %q", tt.args, err, tt.want)
		}
	}
	if _, err := parseWatchArgs([]string{"--below", "20", "x", "-h"}); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("-h: err = %v", err)
	}
}

func TestWatchLoop(t *testing.T) {
	readings := []*battery.Info{
		{Percent: 22, State: battery.Discharging, PowerSource: battery.BatteryPower},
		{Percent: 20, State: battery.Discharging, PowerSource: battery.BatteryPower},
		nil, // a failed read is logged and skipped
		{Percent: 19, State: battery.Discharging, PowerSource: battery.BatteryPower},
		{Percent: 19, State: battery.Charging, PowerSource: battery.AC},
		{Percent: 81, State: battery.Charging, PowerSource: battery.AC},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := 0
	read := func() (*battery.Info, error) {
		info := readings[n]
		n++
		if n == len(readings) {
			cancel() // stop after the last reading
		}
		if info == nil {
			return nil, errors.New("pmset: exit status 1")
		}
		return info, nil
	}
	var ran []string
	hook := func(ctx context.Context, ev watch.Event) error {
		ran = append(ran, ev.String()+"@"+ev.Command)
		if ev.Name == watch.Plug {
			return errors.New("exit status 3")
		}
		return nil
	}
	cfg := watchConfig{
		interval: time.Millisecond,
		hooks: watch.Hooks{
			Thresholds: []watch.Threshold{{Level: 20, Command: "low"}, {Above: true, Level: 80, Command: "high"}},
			OnPlug:     "plugged",
		},
	}
	var logs bytes.Buffer
	if err := watchLoop(ctx, cfg, read, hook, &logs); err != nil {
		t.Fatal(err)
	}
	if want := []string{"below 20@low", "plug@plugged", "above 80@high"}; !reflect.DeepEqual(ran, want) {
		t.Errorf("ran %v, want %v", ran, want)
	}
	for _, want := range []string{"batt: pmset: exit status 1", "batt: plug hook: exit status 3"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log %q missing %q", logs.String(), want)
		}
	}
}

func TestWatchLoopNoBattery(t *testing.T) {
	read := func() (*battery.Info, error) { return nil, battery.ErrNoBattery }
	hook := func(context.Context, watch.Event) error { return nil }
	cfg := watchConfig{interval: time.Millisecond, hooks: watch.Hooks{OnFull: "x"}}
	err := watchLoop(context.Background(), cfg, read, hook, &bytes.Buffer{})
	if !errors.Is(err, battery.ErrNoBattery) {
		t.Errorf("err = %v, want ErrNoBattery", err)
	}
}

func TestShellHookEnv(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	out := filepath.Join(t.TempDir(), "env")
	ev := watch.Event{
		Name:      watch.Below,
		Threshold: 20,
		Command:   `printf '%s|%s|%s|%s|%s' "$BATT_EVENT" "$BATT_PERCENT" "$BATT_STATE" "$BATT_POWER_SOURCE" "$BATT_THRESHOLD" > "$OUT"`,
		Reading:   watch.Reading{Percent: 19, State: battery.Discharging, Source: battery.BatteryPower},
	}
	t.Setenv("OUT", out)
	var stdout, stderr bytes.Buffer
	if err := shellHook(&stdout, &stderr)(context.Background(), ev); err != nil {
		t.Fatalf("%v: %s", err, stderr.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "below|19|discharging|battery|20"; string(got) != want {
		t.Errorf("hook saw %q, want %q", got, want)
	}

	ev.Command = "exit 3"
	if err := shellHook(&stdout, &stderr)(context.Background(), ev); err == nil {
		t.Error("failing hook: want error")
	}
}

func TestWatchUsage(t *testing.T) {
	code, out, _ := runBatt("watch", "-h")
	if code != 0 || !strings.Contains(out, "BATT_PERCENT") {
		t.Errorf("exit %d, output %q", code, out)
	}
	code, _, errOut := runBatt("watch")
	if code != 2 || !strings.Contains(errOut, "nothing to do") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}
