//go:build linux

package battery

import "os"

func readSystem() (*Info, error) {
	dir := os.Getenv("BATTERY_DIR")
	if dir == "" {
		dir = DefaultSysfsDir
	}
	return ReadSysfs(dir)
}
