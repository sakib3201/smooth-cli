//go:build !windows

package supervisor

import (
	"os"
	"strconv"
	"strings"
)

func readProcessResources(pid int) (cpu float64, memMB float64) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(string(data))
	if len(parts) < 24 {
		return 0, 0
	}

	utime, err := strconv.ParseFloat(parts[13], 64)
	if err != nil {
		return 0, 0
	}
	stime, err := strconv.ParseFloat(parts[14], 64)
	if err != nil {
		return 0, 0
	}

	cpu = (utime + stime) / 100.0

	if len(parts) >= 24 {
		rss, err := strconv.ParseFloat(parts[23], 64)
		if err == nil {
			memMB = rss / 1024.0
		}
	}

	return cpu, memMB
}
