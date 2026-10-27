package metrics

import (
	"time"
)

type SystemMetrics struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUUsagePercent float64   `json:"cpu_usage_percent"`
	MemTotalKB      uint64    `json:"mem_total_kb"`
	MemAvailableKB  uint64    `json:"mem_available_kb"`
	MemUsedPercent  float64   `json:"mem_used_percent"`
	DiskTotalKB     uint64    `json:"disk_total_kb,omitempty"`
	DiskUsedKB      uint64    `json:"disk_used_kb,omitempty"`
	DiskPercent     float64   `json:"disk_percent"`
	ProcessCount    int       `json:"process_count"`
	Load1           float64   `json:"load_1"`
	Load5           float64   `json:"load_5"`
	Load15          float64   `json:"load_15"`
	UptimeSeconds   uint64    `json:"uptime_seconds"`
}

type Collector interface {
	Collect() (*SystemMetrics, error)
}
