package concurrency

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// procStatCPUReader reads CPU usage from /proc/stat (Linux).
type procStatCPUReader struct{}

// CPUPercent returns the aggregate CPU usage percentage (0-100).
// Returns error if /proc/stat is unavailable (non-Linux, container without procfs).
func (r *procStatCPUReader) CPUPercent() (float64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		// Parse CPU time fields: user, nice, system, idle, iowait
		parseUint := func(s string) uint64 {
			v, _ := strconv.ParseUint(s, 10, 64)
			return v
		}

		user := parseUint(fields[1])
		nice := parseUint(fields[2])
		system := parseUint(fields[3])
		idle := parseUint(fields[4])
		iowait := uint64(0)
		if len(fields) > 5 {
			iowait = parseUint(fields[5])
		}

		totalIdle := idle + iowait
		totalBusy := user + nice + system
		total := totalBusy + totalIdle

		if total == 0 {
			return 0, nil
		}

		return float64(totalBusy) / float64(total) * 100.0, nil
	}

	return 0, os.ErrNotExist
}

// NoopCPUReader always returns 0 CPU usage. Used for testing or when
// CPU telemetry should be ignored.
type NoopCPUReader struct{}

func (r *NoopCPUReader) CPUPercent() (float64, error) {
	return 0, nil
}

// FixedCPUReader returns a fixed CPU percentage. Used for testing.
type FixedCPUReader struct {
	Percent float64
	Err     error
}

func (r *FixedCPUReader) CPUPercent() (float64, error) {
	return r.Percent, r.Err
}
