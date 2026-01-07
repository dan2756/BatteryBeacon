package battery

import (
	"bytes"
	"fmt"
)

// smartBattery holds the AppleSmartBattery properties batt uses. Fields are
// nil when ioreg doesn't list them.
type smartBattery struct {
	CycleCount        *int64
	DesignCapacity    *int64 // mAh
	MaxCapacity       *int64 // mAh
	CurrentCapacity   *int64 // mAh
	Temperature       *int64 // hundredths of a degree Celsius
	Voltage           *int64 // mV
	Amperage          *int64 // mA, negative while discharging
	ExternalConnected *bool
	PermanentFailure  *int64
}

// parseIoreg parses the output of `ioreg -rn AppleSmartBattery -a`: a plist
// array holding one dict per battery. Machines without a battery print
// nothing, which yields ErrNoBattery.
func parseIoreg(out []byte) (*smartBattery, error) {
	v, err := decodePlist(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("ioreg: %w", err)
	}
	var dict map[string]any
	switch v := v.(type) {
	case nil:
		return nil, ErrNoBattery
	case map[string]any:
		dict = v
	case []any:
		if len(v) == 0 {
			return nil, ErrNoBattery
		}
		d, ok := v[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ioreg: expected a dict, got %T", v[0])
		}
		dict = d
	default:
		return nil, fmt.Errorf("ioreg: unexpected %T", v)
	}
	if installed, ok := dict["BatteryInstalled"].(bool); ok && !installed {
		return nil, ErrNoBattery
	}

	sb := &smartBattery{
		CycleCount:        plistInt(dict, "CycleCount"),
		DesignCapacity:    plistInt(dict, "DesignCapacity"),
		Temperature:       plistInt(dict, "Temperature"),
		Voltage:           plistInt(dict, "Voltage", "AppleRawBatteryVoltage"),
		Amperage:          plistInt(dict, "Amperage", "InstantAmperage"),
		ExternalConnected: plistBool(dict, "ExternalConnected"),
		PermanentFailure:  plistInt(dict, "PermanentFailureStatus"),
	}

	// Apple silicon reports MaxCapacity and CurrentCapacity as percentages
	// and the real mAh figures under AppleRaw*. Intel Macs report mAh in
	// both places.
	sb.MaxCapacity = plistInt(dict, "AppleRawMaxCapacity", "NominalChargeCapacity")
	sb.CurrentCapacity = plistInt(dict, "AppleRawCurrentCapacity")
	if mc := plistInt(dict, "MaxCapacity"); mc != nil && *mc > 100 {
		if sb.MaxCapacity == nil {
			sb.MaxCapacity = mc
		}
		if sb.CurrentCapacity == nil {
			sb.CurrentCapacity = plistInt(dict, "CurrentCapacity")
		}
	}
	return sb, nil
}

// plistInt returns the first of keys that holds an integer.
func plistInt(dict map[string]any, keys ...string) *int64 {
	for _, k := range keys {
		if v, ok := dict[k].(int64); ok {
			return &v
		}
	}
	return nil
}

func plistBool(dict map[string]any, key string) *bool {
	if v, ok := dict[key].(bool); ok {
		return &v
	}
	return nil
}

// apply copies what ioreg knows into in.
func (sb *smartBattery) apply(in *Info) {
	toInt := func(v *int64) *int {
		if v == nil {
			return nil
		}
		return ptr(int(*v))
	}
	in.CycleCount = toInt(sb.CycleCount)
	in.DesignCapacityMAh = toInt(sb.DesignCapacity)
	in.MaxCapacityMAh = toInt(sb.MaxCapacity)
	in.CurrentCapacityMAh = toInt(sb.CurrentCapacity)
	in.AmperageMA = toInt(sb.Amperage)
	if sb.Temperature != nil {
		in.TemperatureC = ptr(round(float64(*sb.Temperature)/100, 1))
	}
	if sb.Voltage != nil {
		in.VoltageV = ptr(round(float64(*sb.Voltage)/1000, 3))
	}
	if sb.PermanentFailure != nil {
		cond := "Normal"
		if *sb.PermanentFailure != 0 {
			cond = "Service Recommended"
		}
		in.Condition = &cond
	}
	if in.PowerSource == "" && sb.ExternalConnected != nil {
		if *sb.ExternalConnected {
			in.PowerSource = AC
		} else {
			in.PowerSource = BatteryPower
		}
	}
}
