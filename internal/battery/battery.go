// Package battery reads the status of a laptop's internal battery.
//
// On macOS it parses `pmset -g batt`; on Linux it reads
// /sys/class/power_supply. The parsers are platform independent so they can
// be tested against fixtures anywhere.
package battery

import (
	"errors"
	"math"
)

// ErrNoBattery is returned on machines without a battery: desktops, VMs,
// CI runners.
var ErrNoBattery = errors.New("no battery found")

// State is a battery's charging state.
type State string

// Charging states, as they appear in JSON output.
const (
	Charging    State = "charging"
	Discharging State = "discharging"
	Charged     State = "charged"
	NotCharging State = "not charging"
)

// PowerSource is what the machine is currently running on.
type PowerSource string

// Power sources, as they appear in JSON output.
const (
	AC           PowerSource = "ac"
	BatteryPower PowerSource = "battery"
)

// Info is a snapshot of battery status. Pointer fields are nil when the
// platform doesn't report them, and encode as JSON null.
type Info struct {
	Percent            int         `json:"percent"`
	State              State       `json:"state"`
	PowerSource        PowerSource `json:"power_source"`
	TimeRemainingMin   *int        `json:"time_remaining_min"`
	TimeToFullMin      *int        `json:"time_to_full_min"`
	CycleCount         *int        `json:"cycle_count"`
	DesignCapacityMAh  *int        `json:"design_capacity_mah"`
	MaxCapacityMAh     *int        `json:"max_capacity_mah"`
	CurrentCapacityMAh *int        `json:"current_capacity_mah"`
	HealthPercent      *float64    `json:"health_percent"`
	TemperatureC       *float64    `json:"temperature_c"`
	VoltageV           *float64    `json:"voltage_v"`
	AmperageMA         *int        `json:"amperage_ma"` // negative while discharging
	Watts              *float64    `json:"watts"`       // negative while discharging
	Condition          *string     `json:"condition"`
}

// Read returns the status of the machine's internal battery, or ErrNoBattery
// if it doesn't have one.
func Read() (*Info, error) {
	return readSystem()
}

// derive fills in fields that can be computed from others.
func (in *Info) derive() {
	if in.HealthPercent == nil && in.MaxCapacityMAh != nil && in.DesignCapacityMAh != nil && *in.DesignCapacityMAh > 0 {
		in.HealthPercent = ptr(round(float64(*in.MaxCapacityMAh)/float64(*in.DesignCapacityMAh)*100, 1))
	}
	if in.Watts == nil && in.VoltageV != nil && in.AmperageMA != nil {
		in.Watts = ptr(round(*in.VoltageV*float64(*in.AmperageMA)/1000, 2))
	}
}

func ptr[T any](v T) *T { return &v }

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

func clampPercent(v int) int {
	return max(0, min(100, v))
}
