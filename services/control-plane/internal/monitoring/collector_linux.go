//go:build linux

package monitoring

import (
	"bufio"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type cpuTimeSample struct {
	idle    uint64
	nonIdle uint64
}

type LinuxCollector struct {
	mu           sync.Mutex
	lastSample   time.Time
	lastCPU      cpuTimeSample
	lastCores    []cpuTimeSample
	lastDiskRead uint64
	lastDiskWrite uint64
	lastNetRx    map[string]uint64
	lastNetTx    map[string]uint64
}

func NewCollector() Collector {
	return &LinuxCollector{
		lastNetRx: make(map[string]uint64),
		lastNetTx: make(map[string]uint64),
	}
}

func (c *LinuxCollector) Collect() (*ServerMetricsSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	var deltaSec float64 = 1.0
	if !c.lastSample.IsZero() {
		deltaSec = now.Sub(c.lastSample).Seconds()
		if deltaSec <= 0 {
			deltaSec = 1.0
		}
	}
	c.lastSample = now

	// 1. CPU
	cpuMetrics := c.collectCPU()

	// 2. Memória
	memMetrics := c.collectMemory()

	// 3. Disco
	diskMetrics := c.collectDisk(deltaSec)

	// 4. Rede
	netMetrics := c.collectNetwork(deltaSec)

	// 5. Visão Geral do Sistema
	sysMetrics := c.collectSystem()

	return &ServerMetricsSnapshot{
		Timestamp: now,
		CPU:       cpuMetrics,
		Memory:    memMetrics,
		Disk:      diskMetrics,
		Network:   netMetrics,
		System:    sysMetrics,
	}, nil
}

func (c *LinuxCollector) collectCPU() CPUMetrics {
	numCPU := runtime.NumCPU()
	metrics := CPUMetrics{
		Cores: numCPU,
	}

	file, err := os.Open("/proc/stat")
	if err != nil {
		return metrics
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var perCore []float64

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)[1:]
			if len(fields) >= 4 {
				idle, nonIdle := parseCPULine(fields)
				totalDiff := float64((idle + nonIdle) - (c.lastCPU.idle + c.lastCPU.nonIdle))
				idleDiff := float64(idle - c.lastCPU.idle)
				c.lastCPU = cpuTimeSample{idle: idle, nonIdle: nonIdle}

				if totalDiff > 0 {
					metrics.TotalPercent = ((totalDiff - idleDiff) / totalDiff) * 100.0
				}
			}
		} else if strings.HasPrefix(line, "cpu") && len(line) > 3 && line[3] >= '0' && line[3] <= '9' {
			fields := strings.Fields(line)[1:]
			if len(fields) >= 4 {
				coreIdx, _ := strconv.Atoi(strings.Fields(line)[0][3:])
				idle, nonIdle := parseCPULine(fields)

				if coreIdx >= len(c.lastCores) {
					c.lastCores = append(c.lastCores, make([]cpuTimeSample, coreIdx-len(c.lastCores)+1)...)
				}
				prev := c.lastCores[coreIdx]
				totalDiff := float64((idle + nonIdle) - (prev.idle + prev.nonIdle))
				idleDiff := float64(idle - prev.idle)
				c.lastCores[coreIdx] = cpuTimeSample{idle: idle, nonIdle: nonIdle}

				var corePercent float64
				if totalDiff > 0 {
					corePercent = ((totalDiff - idleDiff) / totalDiff) * 100.0
				}
				perCore = append(perCore, corePercent)
			}
		}
	}

	metrics.PerCorePercent = perCore
	return metrics
}

func parseCPULine(fields []string) (uint64, uint64) {
	user, _ := strconv.ParseUint(fields[0], 10, 64)
	nice, _ := strconv.ParseUint(fields[1], 10, 64)
	system, _ := strconv.ParseUint(fields[2], 10, 64)
	idle, _ := strconv.ParseUint(fields[3], 10, 64)
	return idle, user + nice + system
}

func (c *LinuxCollector) collectMemory() MemoryMetrics {
	var m MemoryMetrics
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return m
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var swapFree uint64

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(parts[1], 10, 64)
		valBytes := val * 1024 // /proc/meminfo está em kB

		switch parts[0] {
		case "MemTotal:":
			m.TotalBytes = valBytes
		case "MemAvailable:":
			m.AvailableBytes = valBytes
		case "SwapTotal:":
			m.SwapTotalBytes = valBytes
		case "SwapFree:":
			swapFree = valBytes
		}
	}

	if m.TotalBytes > 0 {
		m.UsedBytes = m.TotalBytes - m.AvailableBytes
		m.UsedPercent = (float64(m.UsedBytes) / float64(m.TotalBytes)) * 100.0
	}
	if m.SwapTotalBytes > 0 {
		m.SwapUsedBytes = m.SwapTotalBytes - swapFree
		m.SwapUsedPercent = (float64(m.SwapUsedBytes) / float64(m.SwapTotalBytes)) * 100.0
	}

	return m
}

func (c *LinuxCollector) collectDisk(deltaSec float64) DiskActivity {
	var activity DiskActivity

	// 1. Partição raiz /
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		// #nosec G115 -- stat.Bsize e Blocks são valores positivos retornados por statfs
		total := stat.Blocks * uint64(stat.Bsize)
		// #nosec G115 -- stat.Bsize e Bfree são valores positivos retornados por statfs
		free := stat.Bfree * uint64(stat.Bsize)
		used := total - free
		var usedPct float64
		if total > 0 {
			usedPct = (float64(used) / float64(total)) * 100.0
		}
		activity.Partitions = append(activity.Partitions, DiskPartition{
			MountPoint:  "/",
			Device:      "/dev/root",
			FSType:      "ext4",
			TotalBytes:  total,
			UsedBytes:   used,
			FreeBytes:   free,
			UsedPercent: usedPct,
		})
	}

	// 2. Leitura de throughput via /proc/diskstats
	file, err := os.Open("/proc/diskstats")
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		var totalReadSectors, totalWriteSectors uint64

		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 10 {
				dev := fields[2]
				if strings.HasPrefix(dev, "sd") || strings.HasPrefix(dev, "nvme") || strings.HasPrefix(dev, "vd") {
					readSec, _ := strconv.ParseUint(fields[5], 10, 64)
					writeSec, _ := strconv.ParseUint(fields[9], 10, 64)
					totalReadSectors += readSec
					totalWriteSectors += writeSec
				}
			}
		}

		if c.lastDiskRead > 0 && totalReadSectors >= c.lastDiskRead {
			readBytes := (totalReadSectors - c.lastDiskRead) * 512
			activity.ReadBytesPerSec = uint64(float64(readBytes) / deltaSec)
		}
		if c.lastDiskWrite > 0 && totalWriteSectors >= c.lastDiskWrite {
			writeBytes := (totalWriteSectors - c.lastDiskWrite) * 512
			activity.WriteBytesPerSec = uint64(float64(writeBytes) / deltaSec)
		}

		c.lastDiskRead = totalReadSectors
		c.lastDiskWrite = totalWriteSectors
	}

	return activity
}

func (c *LinuxCollector) collectNetwork(deltaSec float64) []NetworkInterface {
	var list []NetworkInterface

	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return list
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.Split(line, ":")
		ifName := strings.TrimSpace(parts[0])
		if ifName == "lo" {
			continue // descarta loopback
		}

		fields := strings.Fields(parts[1])
		if len(fields) >= 9 {
			rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
			txBytes, _ := strconv.ParseUint(fields[8], 10, 64)

			var rxRate, txRate uint64
			if lastRx, exists := c.lastNetRx[ifName]; exists && rxBytes >= lastRx {
				rxRate = uint64(float64(rxBytes-lastRx) / deltaSec)
			}
			if lastTx, exists := c.lastNetTx[ifName]; exists && txBytes >= lastTx {
				txRate = uint64(float64(txBytes-lastTx) / deltaSec)
			}

			c.lastNetRx[ifName] = rxBytes
			c.lastNetTx[ifName] = txBytes

			list = append(list, NetworkInterface{
				Name:          ifName,
				RxBytesPerSec: rxRate,
				TxBytesPerSec: txRate,
				RxTotalBytes:  rxBytes,
				TxTotalBytes:  txBytes,
			})
		}
	}
	return list
}

func (c *LinuxCollector) collectSystem() SystemOverview {
	hostname, _ := os.Hostname()
	uptime := parseUptime()
	l1, l5, l15, procs := parseLoadAndProcs()
	kernel := parseKernel()
	ip := getLocalIP()
	temp := parseHardwareTemp()

	return SystemOverview{
		Hostname:                  hostname,
		OS:                        "Ubuntu Server",
		KernelVersion:             kernel,
		IPAddress:                 ip,
		UptimeSeconds:             uptime,
		Load1:                     l1,
		Load5:                     l5,
		Load15:                    l15,
		ProcessCount:              procs,
		HardwareTemperatureCelsius: temp,
	}
}

func parseUptime() uint64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		f, _ := strconv.ParseFloat(fields[0], 64)
		return uint64(f)
	}
	return 0
}

func parseLoadAndProcs() (float64, float64, float64, int) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 4 {
		l1, _ := strconv.ParseFloat(fields[0], 64)
		l5, _ := strconv.ParseFloat(fields[1], 64)
		l15, _ := strconv.ParseFloat(fields[2], 64)

		procsParts := strings.Split(fields[3], "/")
		var totalProcs int
		if len(procsParts) >= 2 {
			totalProcs, _ = strconv.Atoi(procsParts[1])
		}
		return l1, l5, l15, totalProcs
	}
	return 0, 0, 0, 0
}

func parseKernel() string {
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	return runtime.GOOS
}

func parseHardwareTemp() *float64 {
	// Procura em /sys/class/thermal/thermal_zone0/temp
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return nil
	}
	milli, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil || milli <= 0 {
		return nil
	}
	celsius := milli / 1000.0
	return &celsius
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
