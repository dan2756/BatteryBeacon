package battery

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParsePmset(t *testing.T) {
	tests := []struct {
		file    string
		percent int
		state   State
		source  PowerSource
		minutes *int
	}{
		{"charging.txt", 87, Charging, AC, ptr(72)},
		{"discharging.txt", 64, Discharging, BatteryPower, ptr(277)},
		{"no-estimate.txt", 71, Discharging, BatteryPower, nil},
		{"charged.txt", 100, Charged, AC, ptr(0)},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			st, err := parsePmset(string(readFixture(t, "pmset/"+tt.file)))
			if err != nil {
				t.Fatal(err)
			}
			if st.Percent != tt.percent {
				t.Errorf("percent = %d, want %d", st.Percent, tt.percent)
			}
			if st.State != tt.state {
				t.Errorf("state = %q, want %q", st.State, tt.state)
			}
			if st.PowerSource != tt.source {
				t.Errorf("source = %q, want %q", st.PowerSource, tt.source)
			}
			if !equalPtr(st.Minutes, tt.minutes) {
				t.Errorf("minutes = %s, want %s", fmtPtr(st.Minutes), fmtPtr(tt.minutes))
			}
		})
	}
}

func TestParsePmsetNoBattery(t *testing.T) {
	_, err := parsePmset(string(readFixture(t, "pmset/desktop.txt")))
	if !errors.Is(err, ErrNoBattery) {
		t.Fatalf("err = %v, want ErrNoBattery", err)
	}

	removed := "Now drawing from 'AC Power'\n -InternalBattery-0 (id=4653155)\t0%; charging; (no estimate) present: false\n"
	if _, err := parsePmset(removed); !errors.Is(err, ErrNoBattery) {
		t.Fatalf("present: false: err = %v, want ErrNoBattery", err)
	}
}

func TestParsePmsetZeroWhileEstimating(t *testing.T) {
	out := "Now drawing from 'Battery Power'\n -InternalBattery-0 (id=4653155)\t93%; discharging; 0:00 remaining present: true\n"
	st, err := parsePmset(out)
	if err != nil {
		t.Fatal(err)
	}
	if st.Minutes != nil {
		t.Errorf("minutes = %d, want nil", *st.Minutes)
	}
}

func TestFromMacTimes(t *testing.T) {
	in, err := fromMac(readFixture(t, "pmset/charging.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if in.TimeRemainingMin != nil || !equalPtr(in.TimeToFullMin, ptr(72)) {
		t.Errorf("charging: remaining=%s to_full=%s", fmtPtr(in.TimeRemainingMin), fmtPtr(in.TimeToFullMin))
	}

	in, err = fromMac(readFixture(t, "pmset/discharging.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !equalPtr(in.TimeRemainingMin, ptr(277)) || in.TimeToFullMin != nil {
		t.Errorf("discharging: remaining=%s to_full=%s", fmtPtr(in.TimeRemainingMin), fmtPtr(in.TimeToFullMin))
	}
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func fmtPtr[T any](p *T) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}
