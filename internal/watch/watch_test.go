package watch

import (
	"reflect"
	"testing"

	"github.com/dan2756/batt/internal/battery"
)

// fires feeds percentages to a watcher and returns the indexes of the
// readings that produced an event, with the events' names.
func fires(w *Watcher, readings []Reading) (idx []int, names []string) {
	for i, r := range readings {
		for _, ev := range w.Observe(r) {
			idx = append(idx, i)
			names = append(names, ev.String())
		}
	}
	return idx, names
}

func percents(state battery.State, src battery.PowerSource, ps ...int) []Reading {
	rs := make([]Reading, len(ps))
	for i, p := range ps {
		rs[i] = Reading{Percent: p, State: state, Source: src}
	}
	return rs
}

func TestBelowHysteresis(t *testing.T) {
	w := New(Hooks{Thresholds: []Threshold{{Level: 20, Command: "low"}}})
	//                                                               0   1   2   3   4   5   6   7   8   9  10
	readings := percents(battery.Discharging, battery.BatteryPower, 25, 21, 20, 19, 18, 20, 21, 22, 21, 20, 19)
	idx, _ := fires(w, readings)
	// Fires at 20, stays quiet while hovering, re-arms at 22, fires again at 20.
	if want := []int{2, 9}; !reflect.DeepEqual(idx, want) {
		t.Errorf("fired at %v, want %v", idx, want)
	}
}

func TestAboveHysteresis(t *testing.T) {
	w := New(Hooks{Thresholds: []Threshold{{Above: true, Level: 80, Command: "unplug"}}})
	//                                                  0   1   2   3   4   5   6   7   8   9
	readings := percents(battery.Charging, battery.AC, 78, 79, 80, 81, 80, 79, 80, 78, 79, 80)
	idx, _ := fires(w, readings)
	// 79 is only one point back, so the threshold re-arms at 78.
	if want := []int{2, 9}; !reflect.DeepEqual(idx, want) {
		t.Errorf("fired at %v, want %v", idx, want)
	}
}

func TestFiresOnFirstReading(t *testing.T) {
	w := New(Hooks{Thresholds: []Threshold{{Level: 20, Command: "low"}}})
	evs := w.Observe(Reading{Percent: 15, State: battery.Discharging, Source: battery.BatteryPower})
	if len(evs) != 1 || evs[0].Name != Below || evs[0].Threshold != 20 || evs[0].Command != "low" {
		t.Fatalf("got %+v", evs)
	}
	if evs[0].Reading.Percent != 15 {
		t.Errorf("event reading = %+v", evs[0].Reading)
	}
	if evs := w.Observe(Reading{Percent: 14, State: battery.Discharging, Source: battery.BatteryPower}); len(evs) != 0 {
		t.Errorf("fired again: %+v", evs)
	}
}

func TestRepeatedThresholds(t *testing.T) {
	w := New(Hooks{
		Thresholds: []Threshold{
			{Level: 20, Command: "low"},
			{Level: 10, Command: "critical"},
			{Above: true, Level: 80, Command: "enough"},
		},
		OnPlug: "plugged",
	})
	//                                                               0   1   2   3  4
	readings := percents(battery.Discharging, battery.BatteryPower, 30, 20, 15, 10, 5)
	//                                                                  5   6   7
	readings = append(readings, percents(battery.Charging, battery.AC, 12, 25, 80)...)
	idx, names := fires(w, readings)
	wantIdx := []int{1, 3, 5, 7}
	wantNames := []string{"below 20", "below 10", "plug", "above 80"}
	if !reflect.DeepEqual(idx, wantIdx) || !reflect.DeepEqual(names, wantNames) {
		t.Errorf("fired %v at %v, want %v at %v", names, idx, wantNames, wantIdx)
	}
}

func TestEventOrder(t *testing.T) {
	w := New(Hooks{
		Thresholds: []Threshold{{Above: true, Level: 80, Command: "a"}, {Above: true, Level: 90, Command: "b"}},
		OnPlug:     "p",
		OnFull:     "f",
	})
	w.Observe(Reading{50, battery.Discharging, battery.BatteryPower})
	_, names := fires(w, []Reading{{100, battery.Charged, battery.AC}})
	if want := []string{"plug", "above 80", "above 90", "full"}; !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestNoHooksNoEvents(t *testing.T) {
	w := New(Hooks{})
	readings := append(percents(battery.Discharging, battery.BatteryPower, 50, 10, 0),
		percents(battery.Charged, battery.AC, 100)...)
	if idx, _ := fires(w, readings); len(idx) != 0 {
		t.Errorf("fired at %v", idx)
	}
}

func TestPlugUnplug(t *testing.T) {
	w := New(Hooks{OnPlug: "p", OnUnplug: "u"})
	readings := []Reading{
		{60, battery.Discharging, battery.BatteryPower}, // first reading: no edge
		{59, battery.Discharging, battery.BatteryPower},
		{59, battery.Charging, battery.AC},
		{61, battery.Charging, battery.AC},
		{61, battery.Discharging, battery.BatteryPower},
		{60, battery.NotCharging, battery.AC},
	}
	idx, names := fires(w, readings)
	if !reflect.DeepEqual(idx, []int{2, 4, 5}) || !reflect.DeepEqual(names, []string{"plug", "unplug", "plug"}) {
		t.Errorf("fired %v at %v", names, idx)
	}
}

func TestFull(t *testing.T) {
	w := New(Hooks{OnFull: "f"})
	readings := []Reading{
		{98, battery.Charging, battery.AC},
		{100, battery.Charged, battery.AC},
		{100, battery.Charged, battery.AC},
		{99, battery.Discharging, battery.BatteryPower},
		{100, battery.Charging, battery.AC}, // 100% on AC counts even before "charged"
		{100, battery.Charged, battery.AC},
		{100, battery.Charged, battery.AC},
	}
	idx, _ := fires(w, readings)
	if want := []int{1, 4}; !reflect.DeepEqual(idx, want) {
		t.Errorf("fired at %v, want %v", idx, want)
	}

	// Already full when watching starts.
	w = New(Hooks{OnFull: "f"})
	if evs := w.Observe(Reading{100, battery.Charged, battery.AC}); len(evs) != 1 || evs[0].Name != Full {
		t.Errorf("got %+v", evs)
	}
}

func TestRearmAtScaleEnds(t *testing.T) {
	// --below 99 can't climb two points past 99. It must re-arm at 100%,
	// and a dip that never gets back there must not fire again.
	w := New(Hooks{Thresholds: []Threshold{{Level: 99, Command: "low"}}})
	readings := percents(battery.Discharging, battery.BatteryPower, 100, 99, 98, 99, 100, 99)
	idx, _ := fires(w, readings)
	if want := []int{1, 5}; !reflect.DeepEqual(idx, want) {
		t.Errorf("--below 99 fired at %v, want %v", idx, want)
	}

	// --above 1 can't fall to -1. It re-arms at 0%.
	w = New(Hooks{Thresholds: []Threshold{{Above: true, Level: 1, Command: "up"}}})
	readings = percents(battery.Charging, battery.AC, 0, 1, 2, 1, 0, 1)
	idx, _ = fires(w, readings)
	if want := []int{1, 5}; !reflect.DeepEqual(idx, want) {
		t.Errorf("--above 1 fired at %v, want %v", idx, want)
	}

	// --below 100 and --above 0 are true for every real charge level, so
	// they fire on the first reading and then stay spent.
	w = New(Hooks{Thresholds: []Threshold{{Level: 100, Command: "always"}}})
	readings = percents(battery.Discharging, battery.BatteryPower, 100, 50, 100, 0)
	idx, _ = fires(w, readings)
	if want := []int{0}; !reflect.DeepEqual(idx, want) {
		t.Errorf("--below 100 fired at %v, want %v", idx, want)
	}
	w = New(Hooks{Thresholds: []Threshold{{Above: true, Level: 0, Command: "always"}}})
	readings = percents(battery.Charging, battery.AC, 0, 50, 0, 100)
	idx, _ = fires(w, readings)
	if want := []int{0}; !reflect.DeepEqual(idx, want) {
		t.Errorf("--above 0 fired at %v, want %v", idx, want)
	}
}

func TestEmpty(t *testing.T) {
	if !(Hooks{}).Empty() {
		t.Error("zero Hooks should be empty")
	}
	if (Hooks{OnFull: "x"}).Empty() {
		t.Error("Hooks with OnFull should not be empty")
	}
}
