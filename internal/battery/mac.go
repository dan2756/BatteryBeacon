package battery

// fromMac builds an Info from the output of `pmset -g batt`.
func fromMac(pmsetOut []byte) (*Info, error) {
	ps, err := parsePmset(string(pmsetOut))
	if err != nil {
		return nil, err
	}
	in := &Info{
		Percent:     ps.Percent,
		State:       ps.State,
		PowerSource: ps.PowerSource,
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
