//go:build !linux

package monitoring

import (
	"math"
	"net"
	"os"
	"runtime"
	"sync"
	"time"
)

type DevCollector struct {
	mu        sync.Mutex
	tickCount uint64
	startTime time.Time
}

func NewCollector() Collector {
	return &DevCollector{
		startTime: time.Now(),
	}
}

func (d *DevCollector) Collect() (*ServerMetricsSnapshot, error) {
	d.mu.Lock()
	d.tickCount++
	t := d.tickCount
	d.mu.Unlock()

	numCPU := runtime.NumCPU()
	hostname, _ := os.Hostname()
	uptime := uint64(time.Since(d.startTime).Seconds())

	// Oscilação suave (senoidal) para gráficos de teste
	baseWave := math.Sin(float64(t)*0.2)*15.0 + 35.0
	cpuPercent := math.Max(5.0, math.Min(95.0, baseWave))

	perCore := make([]float64, numCPU)
	// #nosec G115 -- numCPU is positive core count bounded by host hardware
	for i := uint64(0); i < uint64(numCPU); i++ {
		coreWave := math.Sin(float64(t+i)*0.25)*18.0 + 32.0
		perCore[i] = math.Round(math.Max(2.0, math.Min(98.0, coreWave))*10) / 10
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	totalRAM := uint64(16 * 1024 * 1024 * 1024) // 16 GB simulados
	usedRAM := uint64(6*1024*1024*1024) + uint64(float64(t%10)*50*1024*1024)
	availRAM := totalRAM - usedRAM
	usedPercent := (float64(usedRAM) / float64(totalRAM)) * 100.0

	swapTotal := uint64(4 * 1024 * 1024 * 1024) // 4 GB Swap
	swapUsed := uint64(512 * 1024 * 1024)
	swapPercent := (float64(swapUsed) / float64(swapTotal)) * 100.0

	diskTotal := uint64(256 * 1024 * 1024 * 1024) // 256 GB
	diskUsed := uint64(84 * 1024 * 1024 * 1024)
	diskFree := diskTotal - diskUsed
	diskPct := (float64(diskUsed) / float64(diskTotal)) * 100.0

	readRate := 128*1024 + (t%5)*64*1024
	writeRate := 256*1024 + (t%7)*128*1024

	rxRate := 45*1024 + (t%8)*15*1024
	txRate := 22*1024 + (t%4)*10*1024

	var tempCelsius *float64
	valTemp := 42.5 + math.Sin(float64(t)*0.1)*3.0
	tempCelsius = &valTemp

	ip := "192.168.1.100"
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
				break
			}
		}
	}

	return &ServerMetricsSnapshot{
		Timestamp: time.Now().UTC(),
		CPU: CPUMetrics{
			TotalPercent:   math.Round(cpuPercent*10) / 10,
			PerCorePercent: perCore,
			Cores:          numCPU,
		},
		Memory: MemoryMetrics{
			TotalBytes:      totalRAM,
			UsedBytes:       usedRAM,
			AvailableBytes:  availRAM,
			UsedPercent:     math.Round(usedPercent*10) / 10,
			SwapTotalBytes:  swapTotal,
			SwapUsedBytes:   swapUsed,
			SwapUsedPercent: math.Round(swapPercent*10) / 10,
		},
		Disk: DiskActivity{
			ReadBytesPerSec:  readRate,
			WriteBytesPerSec: writeRate,
			Partitions: []DiskPartition{
				{
					MountPoint:  "/",
					Device:      "/dev/sda1",
					FSType:      "ext4",
					TotalBytes:  diskTotal,
					UsedBytes:   diskUsed,
					FreeBytes:   diskFree,
					UsedPercent: math.Round(diskPct*10) / 10,
				},
			},
		},
		Network: []NetworkInterface{
			{
				Name:          "eth0",
				RxBytesPerSec: rxRate,
				TxBytesPerSec: txRate,
				RxTotalBytes:  t * 1024 * 1024,
				TxTotalBytes:  t * 512 * 1024,
			},
		},
		System: SystemOverview{
			Hostname:                  hostname,
			OS:                        "Linux (Ubuntu 24.04 LTS Compatible)",
			KernelVersion:             "6.8.0-45-generic",
			IPAddress:                 ip,
			UptimeSeconds:             uptime,
			Load1:                     math.Round((0.8+math.Sin(float64(t)*0.1)*0.3)*100) / 100,
			Load5:                     0.75,
			Load15:                    0.65,
			// #nosec G115 -- t % 5 delimitado matematicamente no intervalo [0, 4]
			ProcessCount:              142 + int(t%5),
			HardwareTemperatureCelsius: tempCelsius,
		},
	}, nil
}
