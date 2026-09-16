package battery

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// pmsetStatus is what `pmset -g batt` reports.
type pmsetStatus struct {
	Percent     int
	State       State
	PowerSource PowerSource // empty if pmset didn't say
	Minutes     *int        // to empty or to full; nil while macOS is estimating
}

var (
	pmsetChargeRe  = regexp.MustCompile(`(\d+)%;\s*(.*)$`)
	pmsetTimeRe    = regexp.MustCompile(`^(\d+):(\d{2}) remaining`)
	pmsetPresentRe = regexp.MustCompile(`\s*present: (true|false)\s*$`)
)

// parsePmset parses the output of `pmset -g batt`, which looks like:
//
//	Now drawing from 'AC Power'
//	 -InternalBattery-0 (id=4653155)	87%; charging; 1:12 remaining present: true
//
// Desktops print only the first line; that yields ErrNoBattery.
func parsePmset(out string) (*pmsetStatus, error) {
	var st pmsetStatus
	found := false
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "Now drawing from "):
			src := strings.Trim(strings.TrimPrefix(line, "Now drawing from "), "'")
			if src == "Battery Power" {
				st.PowerSource = BatteryPower
			} else {
				st.PowerSource = AC // 'AC Power' or 'UPS Power'
			}
		case strings.HasPrefix(line, "-InternalBattery") && !found:
			if err := parsePmsetBattery(line, &st); err != nil {
				return nil, err
			}
			found = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNoBattery
	}
	return &st, nil
}

func parsePmsetBattery(line string, st *pmsetStatus) error {
	m := pmsetChargeRe.FindStringSubmatch(line)
	if m == nil {
		return fmt.Errorf("pmset: can't parse %q", line)
	}
	pct, err := strconv.Atoi(m[1])
	if err != nil {
		return fmt.Errorf("pmset: bad percentage in %q", line)
	}
	st.Percent = clampPercent(pct)

	rest := m[2]
	if p := pmsetPresentRe.FindStringSubmatch(rest); p != nil {
		if p[1] == "false" {
			return ErrNoBattery
		}
		rest = rest[:len(rest)-len(p[0])]
	}
	fields := strings.Split(rest, ";")
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}

	switch strings.ToLower(fields[0]) {
	case "charging", "finishing charge":
		st.State = Charging
	case "discharging":
		st.State = Discharging
	case "charged":
		st.State = Charged
	case "ac attached":
		// "87%; AC attached; not charging": plugged in but holding, because
		// of Optimized Battery Charging or a charge limit. There's no time
		// estimate in this form.
		st.State = NotCharging
		st.PowerSource = AC
		return nil
	default:
		return fmt.Errorf("pmset: unrecognized battery state %q", fields[0])
	}

	if len(fields) > 1 {
		if t := pmsetTimeRe.FindStringSubmatch(fields[1]); t != nil {
			h, _ := strconv.Atoi(t[1])
			mm, _ := strconv.Atoi(t[2])
			mins := h*60 + mm
			// Right after plugging or unplugging, pmset can show 0:00 before
			// it has a real estimate. Only trust 0:00 once charged.
			if mins > 0 || st.State == Charged {
				st.Minutes = &mins
			}
		}
		// Anything else, e.g. "(no estimate)", leaves Minutes nil.
	}
	return nil
}
