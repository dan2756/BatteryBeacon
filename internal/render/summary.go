// Package render turns battery.Info into text.
package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/dan2756/batt/internal/battery"
)

// Summary formats in as one human-readable line, e.g.
//
//	87% charging · 1:12 to full · 412 cycles · health 91% · 30.4°C · AC
//
// Parts the platform doesn't report are left out.
func Summary(in *battery.Info) string {
	parts := []string{fmt.Sprintf("%d%% %s", in.Percent, in.State)}
	switch {
	case in.State == battery.Charging && in.TimeToFullMin != nil:
		parts = append(parts, Clock(*in.TimeToFullMin)+" to full")
	case in.State == battery.Discharging && in.TimeRemainingMin != nil:
		parts = append(parts, Clock(*in.TimeRemainingMin)+" remaining")
	}
	if in.CycleCount != nil {
		if *in.CycleCount == 1 {
			parts = append(parts, "1 cycle")
		} else {
			parts = append(parts, fmt.Sprintf("%d cycles", *in.CycleCount))
		}
	}
	if in.HealthPercent != nil {
		parts = append(parts, fmt.Sprintf("health %d%%", int(math.Round(*in.HealthPercent))))
	}
	if in.TemperatureC != nil {
		parts = append(parts, fmt.Sprintf("%.1f°C", *in.TemperatureC))
	}
	if in.PowerSource == battery.AC {
		parts = append(parts, "AC")
	} else {
		parts = append(parts, "battery")
	}
	return strings.Join(parts, " · ")
}

// Clock formats minutes as H:MM.
func Clock(minutes int) string {
	return fmt.Sprintf("%d:%02d", minutes/60, minutes%60)
}
