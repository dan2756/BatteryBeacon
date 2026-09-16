package battery

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadSysfsEnergy(t *testing.T) {
	in, err := ReadSysfs(filepath.Join("testdata", "sysfs", "energy"))
	if err != nil {
		t.Fatal(err)
	}
	want := &Info{
		Percent:            69,
		State:              Discharging,
		PowerSource:        BatteryPower,
		TimeRemainingMin:   ptr(303),
		CycleCount:         ptr(213),
		DesignCapacityMAh:  ptr(4922),
		MaxCapacityMAh:     ptr(4315),
		CurrentCapacityMAh: ptr(2978),
		HealthPercent:      ptr(87.7),
		VoltageV:           ptr(12.201),
		AmperageMA:         ptr(-559),
		Watts:              ptr(-6.82),
	}
	assertInfo(t, in, want)
}

func TestReadSysfsCharge(t *testing.T) {
	// Batteries that report charge_* (µAh) and current_now (µA) instead of
	// energy_* and power_now, as many Dell and HP laptops do.
	in, err := ReadSysfs(filepath.Join("testdata", "sysfs", "charge"))
	if err != nil {
		t.Fatal(err)
	}
	want := &Info{
		Percent:            57,
		State:              Charging,
		PowerSource:        AC,
		TimeToFullMin:      ptr(99),
		CycleCount:         ptr(57),
		DesignCapacityMAh:  ptr(6874),
		MaxCapacityMAh:     ptr(6530),
		CurrentCapacityMAh: ptr(3721),
		HealthPercent:      ptr(95.0),
		TemperatureC:       ptr(31.6),
		VoltageV:           ptr(8.314),
		AmperageMA:         ptr(1702),
		Watts:              ptr(14.15),
		Condition:          ptr("Good"),
	}
	assertInfo(t, in, want)
}

func TestReadSysfsNegativeCurrent(t *testing.T) {
	// Some drivers report current_now as negative while discharging.
	dir := t.TempDir()
	for name, v := range map[string]string{
		"type":        "Battery",
		"status":      "Discharging",
		"charge_now":  "3721000",
		"charge_full": "6530000",
		"current_now": "-1702000",
		"voltage_now": "7912000",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(v+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in, err := ReadSysfs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if in.Percent != 57 {
		t.Errorf("percent = %d, want 57 (from charge_now/charge_full)", in.Percent)
	}
	if !equalPtr(in.AmperageMA, ptr(-1702)) || !equalPtr(in.TimeRemainingMin, ptr(131)) || !equalPtr(in.Watts, ptr(-13.47)) {
		t.Errorf("amperage=%s remaining=%s watts=%s", fmtPtr(in.AmperageMA), fmtPtr(in.TimeRemainingMin), fmtPtr(in.Watts))
	}
	if in.DesignCapacityMAh != nil || in.HealthPercent != nil {
		t.Errorf("design=%s health=%s, want nil", fmtPtr(in.DesignCapacityMAh), fmtPtr(in.HealthPercent))
	}
}

func TestReadSysfsNoDraw(t *testing.T) {
	// Drivers briefly report a zero rate while discharging; watts must come
	// out as 0, not -0.
	for _, files := range []map[string]string{
		{"energy_now": "34000000", "energy_full": "50000000", "power_now": "0"},
		{"energy_now": "34000000", "energy_full": "50000000", "power_now": "3000"},
		{"charge_now": "3721000", "charge_full": "6530000", "current_now": "0"},
	} {
		dir := t.TempDir()
		files["type"] = "Battery"
		files["status"] = "Discharging"
		files["voltage_now"] = "12201000"
		for name, v := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(v+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		in, err := ReadSysfs(dir)
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := json.Marshal(in.Watts); string(b) != "0" {
			t.Errorf("%v: watts = %s, want 0", files, b)
		}
	}
}

func TestReadSysfsBatteryDir(t *testing.T) {
	// BATTERY_DIR may point straight at the battery.
	in, err := ReadSysfs(filepath.Join("testdata", "sysfs", "energy", "BAT0"))
	if err != nil {
		t.Fatal(err)
	}
	if in.Percent != 69 || in.State != Discharging {
		t.Errorf("got %d%% %s", in.Percent, in.State)
	}
	// Without a charger to ask, the source is inferred from the status.
	if in.PowerSource != BatteryPower {
		t.Errorf("power source = %q, want battery", in.PowerSource)
	}
}

func TestReadSysfsNoBattery(t *testing.T) {
	for _, dir := range []string{
		filepath.Join("testdata", "sysfs", "desktop"), // AC adapter and a wireless mouse
		filepath.Join("testdata", "sysfs", "missing"),
	} {
		if _, err := ReadSysfs(dir); !errors.Is(err, ErrNoBattery) {
			t.Errorf("%s: err = %v, want ErrNoBattery", dir, err)
		}
	}
}

func assertInfo(t *testing.T, got, want *Info) {
	t.Helper()
	g, _ := json.MarshalIndent(got, "", "  ")
	w, _ := json.MarshalIndent(want, "", "  ")
	if string(g) != string(w) {
		t.Errorf("got:\n%s\nwant:\n%s", g, w)
	}
}
