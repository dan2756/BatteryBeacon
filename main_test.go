package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dan2756/batt/internal/battery"
)

func ptr[T any](v T) *T { return &v }

func fakeBattery(t *testing.T, info *battery.Info, err error) {
	t.Helper()
	old := readBattery
	readBattery = func() (*battery.Info, error) { return info, err }
	t.Cleanup(func() { readBattery = old })
}

func sample() *battery.Info {
	return &battery.Info{
		Percent:            87,
		State:              battery.Charging,
		PowerSource:        battery.AC,
		TimeToFullMin:      ptr(72),
		CycleCount:         ptr(412),
		DesignCapacityMAh:  ptr(5279),
		MaxCapacityMAh:     ptr(4811),
		CurrentCapacityMAh: ptr(4187),
		HealthPercent:      ptr(91.1),
		TemperatureC:       ptr(30.4),
		VoltageV:           ptr(12.684),
		AmperageMA:         ptr(1834),
		Watts:              ptr(23.26),
	}
}

func runBatt(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestSummaryOutput(t *testing.T) {
	fakeBattery(t, sample(), nil)
	code, out, _ := runBatt()
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "87% charging · 1:12 to full · 412 cycles · health 91% · 30.4°C · AC\n"
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestJSONOutput(t *testing.T) {
	fakeBattery(t, sample(), nil)
	code, out, _ := runBatt("--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := `{
  "percent": 87,
  "state": "charging",
  "power_source": "ac",
  "time_remaining_min": null,
  "time_to_full_min": 72,
  "cycle_count": 412,
  "design_capacity_mah": 5279,
  "max_capacity_mah": 4811,
  "current_capacity_mah": 4187,
  "health_percent": 91.1,
  "temperature_c": 30.4,
  "voltage_v": 12.684,
  "amperage_ma": 1834,
  "watts": 23.26,
  "condition": null
}
`
	if out != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestNoBattery(t *testing.T) {
	fakeBattery(t, nil, battery.ErrNoBattery)
	code, out, errOut := runBatt("--json")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want nothing", out)
	}
	if !strings.Contains(errOut, "no battery found") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestSingleValues(t *testing.T) {
	fakeBattery(t, sample(), nil)
	for cmd, want := range map[string]string{
		"percent": "87\n",
		"state":   "charging\n",
		"time":    "1:12\n",
		"health":  "91\n",
	} {
		code, out, errOut := runBatt(cmd)
		if code != 0 || out != want {
			t.Errorf("batt %s: exit %d, got %q, want %q (stderr %q)", cmd, code, out, want, errOut)
		}
	}
}

func TestFormat(t *testing.T) {
	fakeBattery(t, sample(), nil)
	code, out, _ := runBatt("--format", "{percent}% {state} {time}")
	if code != 0 || out != "87% charging 1:12\n" {
		t.Errorf("exit %d, got %q", code, out)
	}
}

func TestEmptyFormat(t *testing.T) {
	// --format '' is an empty template, not a missing --format.
	fakeBattery(t, sample(), nil)
	if code, out, _ := runBatt("--format", ""); code != 0 || out != "\n" {
		t.Errorf("exit %d, got %q, want an empty line", code, out)
	}
}

func TestBadFormatIsUsageError(t *testing.T) {
	// A bad template is reported before the battery is read, so it is a
	// usage error even on a machine without a battery.
	fakeBattery(t, nil, battery.ErrNoBattery)
	code, _, errOut := runBatt("--format", "{nope}")
	if code != 2 || !strings.Contains(errOut, "unknown field {nope}") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestUsageErrors(t *testing.T) {
	fakeBattery(t, sample(), nil)
	for _, args := range [][]string{
		{"--bogus"},
		{"extra"},
		{"percent", "state"},
		{"--json", "percent"},
		{"--json", "--format", "{percent}"},
		{"--json", "--format", ""},
		{"--format", "", "percent"},
	} {
		if code, _, _ := runBatt(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
	if code, out, _ := runBatt("-h"); code != 0 || !strings.Contains(out, "Usage:") {
		t.Errorf("-h: exit %d, output %q", code, out)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write /dev/stdout: no space left on device")
}

func TestWriteError(t *testing.T) {
	// Output that can't be written is an error in every mode, as with
	// batt > /dev/full.
	fakeBattery(t, sample(), nil)
	for _, args := range [][]string{
		nil,
		{"--json"},
		{"--format", "{percent}"},
		{"percent"},
		{"-h"},
		{"--version"},
		{"watch", "-h"},
	} {
		var errOut bytes.Buffer
		code := run(args, failingWriter{}, &errOut)
		if code != 1 || !strings.Contains(errOut.String(), "no space left on device") {
			t.Errorf("%v: exit %d, stderr %q", args, code, errOut.String())
		}
	}
}
