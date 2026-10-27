//go:build !linux

package metrics

import (
	"time"
)

type StubCollector struct{}

func NewCollector() Collector {
	return &StubCollector{}
}

func (c *StubCollector) Collect() (*SystemMetrics, error) {
	return &SystemMetrics{
		Timestamp:       time.Now().UTC(),
		CPUUsagePercent: 5.0,
		MemTotalKB:      16 * 1024 * 1024,
		MemAvailableKB:  10 * 1024 * 1024,
		MemUsedPercent:  37.5,
		DiskPercent:     45.0,
		ProcessCount:    120,
		Load1:           0.5,
		Load5:           0.4,
		Load15:          0.3,
		UptimeSeconds:   3600,
	}, nil
}
