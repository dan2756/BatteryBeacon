package battery

// fromMac builds an Info from the output of `pmset -g batt` and
// `ioreg -rn AppleSmartBattery -a`. pmset is authoritative for the charge
// level and state (it matches the menu bar); ioreg adds capacities, cycle
// count and electrical readings. ioregOut may be empty.
func fromMac(pmsetOut, ioregOut []byte) (*Info, error) {
	ps, err := parsePmset(string(pmsetOut))
	if err != nil {
		return nil, err
	}
	in := &Info{
		Percent:     ps.Percent,
		State:       ps.State,
		PowerSource: ps.PowerSource,
	}
	// ioreg is a bonus: if it's missing or unreadable, report what pmset
	// knows rather than failing.
	if sb, err := parseIoreg(ioregOut); err == nil {
		sb.apply(in)
	}
	if in.PowerSource == "" {
		if in.State == Discharging {
			in.PowerSource = BatteryPower
		} else {
			in.PowerSource = AC
		}
	}
	switch in.State {
	case Charging:
		in.TimeToFullMin = ps.Minutes
	case Charged:
		in.TimeToFullMin = ptr(0)
	case Discharging:
		in.TimeRemainingMin = ps.Minutes
	}
	in.derive()
	return in, nil
}
