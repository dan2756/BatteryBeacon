// Package watch decides when battery hooks fire, given a series of readings.
// It does no I/O, so its behavior can be tested with plain sequences.
package watch

import (
	"fmt"

	"github.com/dan2756/batt/internal/battery"
)

// Rearm is how many percentage points the charge must move back across a
// threshold before that threshold can fire again. Without it, a battery
// hovering at 20% would fire --below 20 on every other reading.
const Rearm = 2

// Threshold runs Command when the charge reaches Level: at or below it, or
// at or above it if Above is set.
type Threshold struct {
	Above   bool
	Level   int
	Command string
}

// Hooks are the commands to run. Empty commands are ignored.
type Hooks struct {
	Thresholds []Threshold
	OnPlug     string
	OnUnplug   string
	OnFull     string
}

// Empty reports whether no hook is set.
func (h Hooks) Empty() bool {
	return len(h.Thresholds) == 0 && h.OnPlug == "" && h.OnUnplug == "" && h.OnFull == ""
}

// Reading is the part of battery.Info that hooks depend on.
type Reading struct {
	Percent int
	State   battery.State
	Source  battery.PowerSource
}

// ReadingOf extracts a Reading from in.
func ReadingOf(in *battery.Info) Reading {
	return Reading{Percent: in.Percent, State: in.State, Source: in.PowerSource}
}

// Event names, as passed to hooks in BATT_EVENT.
const (
	Below  = "below"
	Above  = "above"
	Plug   = "plug"
	Unplug = "unplug"
	Full   = "full"
)

// Event is a hook that should run.
type Event struct {
	Name      string
	Threshold int // the threshold's level, for Below and Above
	Command   string
	Reading   Reading
}

func (e Event) String() string {
	if e.Name == Below || e.Name == Above {
		return fmt.Sprintf("%s %d", e.Name, e.Threshold)
	}
	return e.Name
}

// Watcher turns readings into events.
//
// Thresholds and OnFull are levels: each fires when its condition becomes
// true, including on the first reading, and then not again until it has
// been false (for thresholds, by Rearm points, or at 0% or 100% when that
// margin would fall off the scale). OnPlug and OnUnplug are edges and fire
// only when the power source changes between readings.
type Watcher struct {
	hooks     Hooks
	armed     []bool
	fullArmed bool
	last      *Reading
}

// New returns a Watcher with every hook armed.
func New(h Hooks) *Watcher {
	w := &Watcher{hooks: h, armed: make([]bool, len(h.Thresholds)), fullArmed: true}
	for i := range w.armed {
		w.armed[i] = true
	}
	return w
}

// Observe records a reading and returns the events it triggers, in the
// order: plug/unplug, thresholds as given, full.
func (w *Watcher) Observe(r Reading) []Event {
	var evs []Event
	emit := func(name string, level int, cmd string) {
		if cmd != "" {
			evs = append(evs, Event{Name: name, Threshold: level, Command: cmd, Reading: r})
		}
	}

	if w.last != nil && w.last.Source != r.Source {
		switch r.Source {
		case battery.AC:
			emit(Plug, 0, w.hooks.OnPlug)
		case battery.BatteryPower:
			emit(Unplug, 0, w.hooks.OnUnplug)
		}
	}

	for i, t := range w.hooks.Thresholds {
		if !w.armed[i] {
			w.armed[i] = t.rearmed(r.Percent)
			continue
		}
		if t.reached(r.Percent) {
			w.armed[i] = false
			name := Below
			if t.Above {
				name = Above
			}
			emit(name, t.Level, t.Command)
		}
	}

	full := r.State == battery.Charged || (r.Percent >= 100 && r.Source == battery.AC)
	switch {
	case full && w.fullArmed:
		w.fullArmed = false
		emit(Full, 0, w.hooks.OnFull)
	case !full:
		w.fullArmed = true
	}

	w.last = &r
	return evs
}

func (t Threshold) reached(pct int) bool {
	if t.Above {
		return pct >= t.Level
	}
	return pct <= t.Level
}

// rearmed reports whether pct has moved far enough back from t for t to
// fire again. The margin is Rearm points, but a charge level only runs
// from 0 to 100, so a threshold closer to an end than that re-arms there:
// --below 99 re-arms at 100%, not at an unreachable 101%. An always-true
// threshold (--below 100, --above 0) never re-arms, because no reading
// makes it false.
func (t Threshold) rearmed(pct int) bool {
	if t.Above {
		if pct <= t.Level-Rearm {
			return true
		}
		return t.Level > 0 && pct <= 0
	}
	if pct >= t.Level+Rearm {
		return true
	}
	return t.Level < 100 && pct >= 100
}
