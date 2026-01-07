//go:build darwin

package battery

import (
	"fmt"
	"os/exec"
)

func readSystem() (*Info, error) {
	pmset, err := exec.Command("/usr/bin/pmset", "-g", "batt").Output()
	if err != nil {
		return nil, fmt.Errorf("pmset: %w", err)
	}
	// ioreg only adds detail; without it batt still has pmset's basics.
	ioreg, err := exec.Command("/usr/sbin/ioreg", "-rn", "AppleSmartBattery", "-a").Output()
	if err != nil {
		ioreg = nil
	}
	return fromMac(pmset, ioreg)
}
