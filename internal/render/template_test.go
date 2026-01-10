package render

import (
	"strings"
	"testing"

	"github.com/dan2756/batt/internal/battery"
)

func charging() *battery.Info {
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
		Condition:          ptr("Normal"),
	}
}

func discharging() *battery.Info {
	return &battery.Info{
		Percent:          64,
		State:            battery.Discharging,
		PowerSource:      battery.BatteryPower,
		TimeRemainingMin: ptr(277),
		AmperageMA:       ptr(-1207),
		Watts:            ptr(-14.33),
	}
}

func TestTemplate(t *testing.T) {
	tests := []struct {
		format string
		in     *battery.Info
		want   string
	}{
		{"{percent}% {state}", charging(), "87% charging"},
		{"{percent}% {state}", discharging(), "64% discharging"},
		{"{time}", charging(), "1:12"},
		{"{time}", discharging(), "4:37"},
		{"{health}% ({health_percent})", charging(), "91% (91.1)"},
		{"{watts}W {amperage_ma}mA {voltage_v}V", charging(), "23.26W 1834mA 12.684V"},
		{"{watts}W {amperage_ma}mA", discharging(), "-14.33W -1207mA"},
		{"{current_capacity_mah}/{max_capacity_mah}/{design_capacity_mah}", charging(), "4187/4811/5279"},
		{"{cycle_count} {temperature_c} {condition} {power_source}", charging(), "412 30.4 Normal ac"},
		{"{time_remaining_min}|{time_to_full_min}", charging(), "|72"},
		// Unknown values render empty.
		{"[{health}] [{temperature_c}] [{condition}] [{cycle_count}]", discharging(), "[] [] [] []"},
		{"{ percent }", charging(), "87"},
		{"{{percent}} = {percent}", charging(), "{percent} = 87"},
		{"no fields", charging(), "no fields"},
		{"", charging(), ""},
		{"⚡{percent}%", charging(), "⚡87%"},
	}
	for _, tt := range tests {
		tmpl, err := ParseTemplate(tt.format)
		if err != nil {
			t.Errorf("%q: %v", tt.format, err)
			continue
		}
		if got := tmpl.Execute(tt.in); got != tt.want {
			t.Errorf("%q: got %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestTemplateErrors(t *testing.T) {
	tests := map[string]string{
		"{percnt}":    "unknown field {percnt}",
		"{percent":    "unclosed {",
		"100%}":       "unexpected }",
		"{}":          "unknown field {}",
		"{state} {x}": "unknown field {x}",
	}
	for format, want := range tests {
		_, err := ParseTemplate(format)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", format, err, want)
		}
	}
}

func TestFieldsAreAllKnown(t *testing.T) {
	for _, name := range Fields {
		if _, ok := Field(charging(), name); !ok {
			t.Errorf("Fields lists %q but Field doesn't know it", name)
		}
	}
}
