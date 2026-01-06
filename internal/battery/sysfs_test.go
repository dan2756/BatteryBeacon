package battery

import (
	"encoding/json"
	"errors"
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
