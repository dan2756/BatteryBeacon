//go:build !darwin && !linux

package battery

import (
	"fmt"
	"runtime"
)

func readSystem() (*Info, error) {
	return nil, fmt.Errorf("reading the battery is not supported on %s", runtime.GOOS)
}
