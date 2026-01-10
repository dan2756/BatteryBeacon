package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dan2756/batt/internal/battery"
)

// Fields lists every name a template can use, in the order help shows them.
var Fields = []string{
	"percent", "state", "power_source", "time", "health",
	"time_remaining_min", "time_to_full_min", "cycle_count",
	"design_capacity_mah", "max_capacity_mah", "current_capacity_mah",
	"health_percent", "temperature_c", "voltage_v", "amperage_ma", "watts",
	"condition",
}

// Field returns one value of in as text. Values the platform doesn't report
// are empty. Besides the JSON field names it knows:
//
//	time    H:MM until empty, or until full while charging
//	health  health_percent rounded to a whole number
func Field(in *battery.Info, name string) (string, bool) {
	switch name {
	case "percent":
		return strconv.Itoa(in.Percent), true
	case "state":
		return string(in.State), true
	case "power_source":
		return string(in.PowerSource), true
	case "time":
		switch {
		case in.TimeRemainingMin != nil:
			return Clock(*in.TimeRemainingMin), true
		case in.TimeToFullMin != nil:
			return Clock(*in.TimeToFullMin), true
		}
		return "", true
	case "health":
		if in.HealthPercent == nil {
			return "", true
		}
		return strconv.Itoa(int(math.Round(*in.HealthPercent))), true
	case "time_remaining_min":
		return intStr(in.TimeRemainingMin), true
	case "time_to_full_min":
		return intStr(in.TimeToFullMin), true
	case "cycle_count":
		return intStr(in.CycleCount), true
	case "design_capacity_mah":
		return intStr(in.DesignCapacityMAh), true
	case "max_capacity_mah":
		return intStr(in.MaxCapacityMAh), true
	case "current_capacity_mah":
		return intStr(in.CurrentCapacityMAh), true
	case "health_percent":
		return floatStr(in.HealthPercent), true
	case "temperature_c":
		return floatStr(in.TemperatureC), true
	case "voltage_v":
		return floatStr(in.VoltageV), true
	case "amperage_ma":
		return intStr(in.AmperageMA), true
	case "watts":
		return floatStr(in.Watts), true
	case "condition":
		if in.Condition == nil {
			return "", true
		}
		return *in.Condition, true
	}
	return "", false
}

func intStr(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func floatStr(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

// Template is a parsed --format string such as "{percent}% {state}".
type Template struct {
	parts []part
}

type part struct {
	text  string
	field string // empty for literal text
}

// ParseTemplate parses s. Fields are written {name}; {{ and }} are literal
// braces.
func ParseTemplate(s string) (*Template, error) {
	t := &Template{}
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			t.parts = append(t.parts, part{text: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '{' && strings.HasPrefix(s[i:], "{{"):
			lit.WriteByte('{')
			i++
		case c == '}' && strings.HasPrefix(s[i:], "}}"):
			lit.WriteByte('}')
			i++
		case c == '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return nil, fmt.Errorf("unclosed { in format at column %d", i+1)
			}
			name := strings.TrimSpace(s[i+1 : i+end])
			if _, ok := Field(&battery.Info{}, name); !ok {
				return nil, fmt.Errorf("unknown field {%s} in format (fields: %s)", name, strings.Join(Fields, ", "))
			}
			flush()
			t.parts = append(t.parts, part{field: name})
			i += end
		case c == '}':
			return nil, fmt.Errorf("unexpected } in format at column %d (use }} for a literal brace)", i+1)
		default:
			lit.WriteByte(c)
		}
	}
	flush()
	return t, nil
}

// Execute fills in t's fields from in.
func (t *Template) Execute(in *battery.Info) string {
	var b strings.Builder
	for _, p := range t.parts {
		if p.field == "" {
			b.WriteString(p.text)
			continue
		}
		v, _ := Field(in, p.field)
		b.WriteString(v)
	}
	return b.String()
}
