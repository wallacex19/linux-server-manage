//go:build linux

package metrics

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type LinuxCollector struct {
	lastIdleTotal uint64
	lastNonIdle   uint64
}

func NewCollector() Collector {
	return &LinuxCollector{}
}

func (c *LinuxCollector) Collect() (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now().UTC(),
	}

	// 1. Memória via /proc/meminfo
	memTotal, memAvail, err := parseMemInfo()
	if err == nil {
		metrics.MemTotalKB = memTotal
		metrics.MemAvailableKB = memAvail
		if memTotal > 0 {
			used := memTotal - memAvail
			metrics.MemUsedPercent = (float64(used) / float64(memTotal)) * 100.0
		}
	}

	// 2. CPU via /proc/stat
	cpuPercent, err := c.calculateCPUUsage()
	if err == nil {
		metrics.CPUUsagePercent = cpuPercent
	}

	// 3. Load average via /proc/loadavg
	load1, load5, load15, err := parseLoadAvg()
	if err == nil {
		metrics.Load1 = load1
		metrics.Load5 = load5
		metrics.Load15 = load15
	}

	// 4. Uptime via /proc/uptime
	uptime, err := parseUptime()
	if err == nil {
		metrics.UptimeSeconds = uptime
	}

	// 5. Disk usage via statfs on /
	diskPercent, totalKB, usedKB, err := getDiskUsage("/")
	if err == nil {
		metrics.DiskPercent = diskPercent
		metrics.DiskTotalKB = totalKB
		metrics.DiskUsedKB = usedKB
	}

	// 6. Process count via /proc
	procCount, err := countProcesses()
	if err == nil {
		metrics.ProcessCount = procCount
	}

	return metrics, nil
}

func getDiskUsage(path string) (float64, uint64, uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}
	// #nosec G115 -- stat.Bsize e Blocks são positivos reportados pelo kernel
	total := stat.Blocks * uint64(stat.Bsize)
	// #nosec G115 -- stat.Bsize e Bfree são positivos reportados pelo kernel
	free := stat.Bfree * uint64(stat.Bsize)
	if total == 0 {
		return 0, 0, 0, nil
	}
	used := total - free
	percent := (float64(used) / float64(total)) * 100.0
	return percent, total / 1024, used / 1024, nil
}

func countProcesses() (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				count++
			}
		}
	}
	return count, nil
}

func parseMemInfo() (uint64, uint64, error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var total, avail uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseKbValue(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			avail = parseKbValue(line)
		}
		if total > 0 && avail > 0 {
			break
		}
	}
	return total, avail, nil
}

func parseKbValue(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		return val
	}
	return 0
}

func (c *LinuxCollector) calculateCPUUsage() (float64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)[1:]
			if len(fields) < 4 {
				return 0, fmt.Errorf("dados insuficientes em /proc/stat")
			}
			user, _ := strconv.ParseUint(fields[0], 10, 64)
			nice, _ := strconv.ParseUint(fields[1], 10, 64)
			system, _ := strconv.ParseUint(fields[2], 10, 64)
			idle, _ := strconv.ParseUint(fields[3], 10, 64)

			idleTotal := idle
			nonIdle := user + nice + system

			total := idleTotal + nonIdle
			prevTotal := c.lastIdleTotal + c.lastNonIdle

			totalDiff := float64(total - prevTotal)
			idleDiff := float64(idleTotal - c.lastIdleTotal)

			c.lastIdleTotal = idleTotal
			c.lastNonIdle = nonIdle

			if prevTotal == 0 || totalDiff == 0 {
				return 0.0, nil
			}

			cpuUsage := ((totalDiff - idleDiff) / totalDiff) * 100.0
			return cpuUsage, nil
		}
	}
	return 0, fmt.Errorf("linha 'cpu' nao encontrada em /proc/stat")
}

func parseLoadAvg() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("formato invalido em /proc/loadavg")
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15, nil
}

func parseUptime() (uint64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("formato invalido em /proc/uptime")
	}
	upFloat, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}
	return uint64(upFloat), nil
}
