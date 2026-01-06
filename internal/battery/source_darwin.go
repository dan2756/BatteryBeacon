//go:build darwin

package battery

import (
	"fmt"
	"os/exec"
)

func readSystem() (*Info, error) {
	out, err := exec.Command("/usr/bin/pmset", "-g", "batt").Output()
	if err != nil {
		return nil, fmt.Errorf("pmset: %w", err)
	}
	return fromMac(out)
}
