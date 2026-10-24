package battery

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultSysfsDir is where Linux exposes batteries and chargers.
const DefaultSysfsDir = "/sys/class/power_supply"

// ReadSysfs reads the first battery under dir, a directory laid out like
// /sys/class/power_supply (BAT0/, AC/, ...). dir may also be a single
// battery's directory. Both energy_* and charge_* batteries are supported.
func ReadSysfs(dir string) (*Info, error) {
	bat, err := findSysfsBattery(dir)
	if err != nil {
		return nil, err
	}
	return readSysfsBattery(bat, chargerOnline(dir))
}

// sysfsDir is one power supply's directory.
type sysfsDir string

func (d sysfsDir) str(name string) string {
	b, err := os.ReadFile(filepath.Join(string(d), name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// num reads an integer attribute, or nil if it's missing or unreadable.
func (d sysfsDir) num(name string) *float64 {
	s := d.str(name)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return ptr(float64(v))
}

func findSysfsBattery(dir string) (sysfsDir, error) {
	if d := sysfsDir(dir); d.str("type") == "Battery" {
		return d, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrNoBattery
	}
	if err != nil {
		return "", err
	}
	// Entries come back sorted, so BAT0 is preferred over BAT1.
	for _, e := range entries {
		d := sysfsDir(filepath.Join(dir, e.Name()))
		if d.str("type") != "Battery" {
			continue
		}
		if d.str("scope") == "Device" {
			continue // a mouse or keyboard, not the system battery
		}
		if d.str("present") == "0" {
			continue // empty bay
		}
		return d, nil
	}
	return "", ErrNoBattery
}

// chargerType reports whether t is a Linux power_supply type that delivers
// external power: every kernel type except Battery and Unknown.
func chargerType(t string) bool {
	switch t {
	case "Mains", "UPS", "USB", "USB_DCP", "USB_CDP", "USB_ACA",
		"USB_C", "USB_PD", "USB_PD_DRP", "BrickID", "Wireless":
		return true
	default:
		return false
	}
}

// chargerOnline reports whether an external supply under dir is online.
// It returns nil when nothing there can say: no charger, or only supplies
// with no online file. Callers then infer the source from battery status.
// One supply that is online wins over any that are not.
func chargerOnline(dir string) *bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var offline *bool
	for _, e := range entries {
		d := sysfsDir(filepath.Join(dir, e.Name()))
		if !chargerType(d.str("type")) {
			continue
		}
		// A supply with no online file is not a charger we can ask.
		// Treating a missing file as 0 would mark the machine unplugged.
		switch d.str("online") {
		case "1":
			on := true
			return &on
		case "0":
			off := false
			offline = &off
		}
	}
	return offline
}

func readSysfsBattery(d sysfsDir, charger *bool) (*Info, error) {
	in := &Info{}
	status := d.str("status")

	switch {
	case charger != nil && *charger:
		in.PowerSource = AC
	case charger != nil:
		in.PowerSource = BatteryPower
	case status == "Discharging":
		in.PowerSource = BatteryPower
	default:
		in.PowerSource = AC
	}

	switch status {
	case "Charging":
		in.State = Charging
	case "Discharging":
		in.State = Discharging
	case "Full":
		in.State = Charged
	case "Not charging":
		in.State = NotCharging
	default:
		// "Unknown": some firmware reports this while a charge threshold
		// holds the battery on AC.
		if in.PowerSource == AC {
			in.State = NotCharging
		} else {
			in.State = Discharging
		}
	}

	// Fill levels and the rate they change at, in the battery's own units.
	// Most report energy (µWh, with power_now in µW); some report charge
	// (µAh, with current_now in µA) instead.
	now, full, design, rate := d.num("energy_now"), d.num("energy_full"), d.num("energy_full_design"), d.num("power_now")
	charge := now == nil && full == nil
	if charge {
		now, full, design, rate = d.num("charge_now"), d.num("charge_full"), d.num("charge_full_design"), d.num("current_now")
	}
	voltage := d.num("voltage_now") // µV
	designVoltage := d.num("voltage_min_design")
	if designVoltage == nil {
		designVoltage = voltage
	}

	switch pct := d.num("capacity"); {
	case pct != nil:
		in.Percent = clampPercent(int(*pct))
	case now != nil && full != nil && *full > 0:
		in.Percent = clampPercent(int(math.Round(*now / *full * 100)))
	default:
		return nil, fmt.Errorf("%s: battery reports no charge level", d)
	}

	// Energy converts to charge at the battery's design voltage, which is
	// how the manufacturer's mAh rating is defined.
	mAh := func(level *float64) *int {
		switch {
		case level == nil:
			return nil
		case charge:
			return ptr(int(math.Round(*level / 1000)))
		case designVoltage == nil || *designVoltage <= 0:
			return nil
		}
		return ptr(int(math.Round(*level / *designVoltage * 1000)))
	}
	in.CurrentCapacityMAh = mAh(now)
	in.MaxCapacityMAh = mAh(full)
	in.DesignCapacityMAh = mAh(design)
	if full != nil && design != nil && *design > 0 {
		in.HealthPercent = ptr(round(*full / *design * 100, 1))
	}

	sign := 1.0
	if in.State == Discharging {
		sign = -1
	}
	if voltage != nil {
		in.VoltageV = ptr(round(*voltage/1e6, 3))
	}
	if rate != nil {
		// Some drivers report the rate as negative while discharging,
		// others never do; take the sign from the status instead.
		r := math.Abs(*rate)
		if charge {
			in.AmperageMA = ptr(int(math.Round(sign * r / 1000)))
			if voltage != nil {
				v := *voltage
				in.Watts = ptr(round(sign*r*v/1e12, 2))
			}
		} else {
			in.Watts = ptr(round(sign*r/1e6, 2))
			if voltage != nil && *voltage > 0 {
				in.AmperageMA = ptr(int(math.Round(sign * r / *voltage * 1000)))
			}
		}
		if r > 0 && now != nil {
			switch in.State {
			case Discharging:
				in.TimeRemainingMin = ptr(int(math.Round(*now / r * 60)))
			case Charging:
				if full != nil && *full > *now {
					in.TimeToFullMin = ptr(int(math.Round((*full - *now) / r * 60)))
				}
			}
		}
	}
	if in.State == Charged {
		in.TimeToFullMin = ptr(0)
	}

	if c := d.num("cycle_count"); c != nil {
		in.CycleCount = ptr(int(*c))
	}
	if t := d.num("temp"); t != nil {
		in.TemperatureC = ptr(round(*t/10, 1)) // tenths of a degree
	}
	if h := d.str("health"); h != "" {
		in.Condition = &h
	}
	in.derive()
	return in, nil
}
