package render

import (
	"testing"

	"github.com/dan2756/batt/internal/battery"
)

func ptr[T any](v T) *T { return &v }

func TestSummary(t *testing.T) {
	tests := []struct {
		name string
		in   battery.Info
		want string
	}{
		{
			name: "charging",
			in: battery.Info{
				Percent: 87, State: battery.Charging, PowerSource: battery.AC,
				TimeToFullMin: ptr(72), CycleCount: ptr(412), HealthPercent: ptr(91.1),
				TemperatureC: ptr(30.4),
			},
			want: "87% charging · 1:12 to full · 412 cycles · health 91% · 30.4°C · AC",
		},
		{
			name: "discharging",
			in: battery.Info{
				Percent: 64, State: battery.Discharging, PowerSource: battery.BatteryPower,
				TimeRemainingMin: ptr(277), CycleCount: ptr(1),
			},
			want: "64% discharging · 4:37 remaining · 1 cycle · battery",
		},
		{
			name: "estimating",
			in:   battery.Info{Percent: 71, State: battery.Discharging, PowerSource: battery.BatteryPower},
			want: "71% discharging · battery",
		},
		{
			name: "charged",
			in: battery.Info{
				Percent: 100, State: battery.Charged, PowerSource: battery.AC,
				TimeToFullMin: ptr(0), HealthPercent: ptr(87.5),
			},
			want: "100% charged · health 88% · AC",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Summary(&tt.in); got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestClock(t *testing.T) {
	for mins, want := range map[int]string{0: "0:00", 5: "0:05", 72: "1:12", 600: "10:00"} {
		if got := Clock(mins); got != want {
			t.Errorf("Clock(%d) = %q, want %q", mins, got, want)
		}
	}
}
