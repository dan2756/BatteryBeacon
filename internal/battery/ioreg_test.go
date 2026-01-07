package battery

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestFromMacCharging(t *testing.T) {
	in, err := fromMac(readFixture(t, "pmset/charging.txt"), readFixture(t, "ioreg/charging.plist"))
	if err != nil {
		t.Fatal(err)
	}
	assertInfo(t, in, &Info{
		Percent:            87,
		State:              Charging,
		PowerSource:        AC,
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
	})
}

func TestFromMacDischarging(t *testing.T) {
	in, err := fromMac(readFixture(t, "pmset/discharging.txt"), readFixture(t, "ioreg/discharging.plist"))
	if err != nil {
		t.Fatal(err)
	}
	assertInfo(t, in, &Info{
		Percent:            64,
		State:              Discharging,
		PowerSource:        BatteryPower,
		TimeRemainingMin:   ptr(277),
		CycleCount:         ptr(412),
		DesignCapacityMAh:  ptr(5279),
		MaxCapacityMAh:     ptr(4811),
		CurrentCapacityMAh: ptr(3079),
		HealthPercent:      ptr(91.1),
		TemperatureC:       ptr(29.1),
		VoltageV:           ptr(11.876),
		AmperageMA:         ptr(-1207), // ioreg prints it as an unsigned 64-bit number
		Watts:              ptr(-14.33),
		Condition:          ptr("Normal"),
	})
}

func TestFromMacWithoutIoreg(t *testing.T) {
	for _, ioreg := range []string{"", "not a plist <"} {
		in, err := fromMac(readFixture(t, "pmset/no-estimate.txt"), []byte(ioreg))
		if err != nil {
			t.Fatalf("ioreg %q: %v", ioreg, err)
		}
		assertInfo(t, in, &Info{Percent: 71, State: Discharging, PowerSource: BatteryPower})
	}
}

func TestParseIoregIntel(t *testing.T) {
	// Intel Macs report capacities in mAh under MaxCapacity/CurrentCapacity.
	sb, err := parseIoreg(readFixture(t, "ioreg/intel.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if !equalPtr(sb.MaxCapacity, ptr[int64](5844)) || !equalPtr(sb.CurrentCapacity, ptr[int64](5844)) {
		t.Errorf("max=%s current=%s, want 5844", fmtPtr(sb.MaxCapacity), fmtPtr(sb.CurrentCapacity))
	}
	in := &Info{}
	sb.apply(in)
	in.derive()
	if !equalPtr(in.HealthPercent, ptr(66.5)) {
		t.Errorf("health = %s, want 66.5", fmtPtr(in.HealthPercent))
	}
}

func TestParseIoregNoBattery(t *testing.T) {
	empty := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>BatteryInstalled</key>
	<false/>
</dict>
</plist>`
	for _, out := range []string{"", "\n", empty} {
		if _, err := parseIoreg([]byte(out)); !errors.Is(err, ErrNoBattery) {
			t.Errorf("%q: err = %v, want ErrNoBattery", out, err)
		}
	}
}

func TestJSONOmitsSerial(t *testing.T) {
	in, err := fromMac(readFixture(t, "pmset/charging.txt"), readFixture(t, "ioreg/charging.plist"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); strings.Contains(strings.ToLower(s), "serial") || strings.Contains(s, "F8Y3091C0DQ1J4A5L") {
		t.Errorf("JSON leaks the serial number: %s", s)
	}
}

func TestDecodePlist(t *testing.T) {
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>a</key><integer>-3</integer>
	<key>b</key><real>2.5</real>
	<key>c</key><array><string> x </string><true/><false/></array>
	<key>d</key><dict/>
	<key>e</key><data>AAEC</data>
	<key>f</key><integer>18446744073709551615</integer>
</dict>
</plist>`
	got, err := decodePlist(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"a": int64(-3),
		"b": 2.5,
		"c": []any{"x", true, false},
		"d": map[string]any{},
		"e": nil,
		"f": int64(-1),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}

	if _, err := decodePlist(strings.NewReader(`<plist><dict><integer>1</integer></dict></plist>`)); err == nil {
		t.Error("value without key: want error")
	}
}
