package monitoring

import "time"

type CPUMetrics struct {
	TotalPercent   float64   `json:"total_percent"`
	PerCorePercent []float64 `json:"per_core_percent,omitempty"`
	Cores          int       `json:"cores"`
}

type MemoryMetrics struct {
	TotalBytes      uint64  `json:"total_bytes"`
	UsedBytes       uint64  `json:"used_bytes"`
	AvailableBytes  uint64  `json:"available_bytes"`
	UsedPercent     float64 `json:"used_percent"`
	SwapTotalBytes  uint64  `json:"swap_total_bytes"`
	SwapUsedBytes   uint64  `json:"swap_used_bytes"`
	SwapUsedPercent float64 `json:"swap_used_percent"`
}

type DiskPartition struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device"`
	FSType      string  `json:"fs_type"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskActivity struct {
	ReadBytesPerSec  uint64          `json:"read_bytes_per_sec"`
	WriteBytesPerSec uint64          `json:"write_bytes_per_sec"`
	Partitions       []DiskPartition `json:"partitions"`
}

type NetworkInterface struct {
	Name           string `json:"name"`
	RxBytesPerSec  uint64 `json:"rx_bytes_per_sec"`
	TxBytesPerSec  uint64 `json:"tx_bytes_per_sec"`
	RxTotalBytes   uint64 `json:"rx_total_bytes"`
	TxTotalBytes   uint64 `json:"tx_total_bytes"`
}

type SystemOverview struct {
	Hostname                  string   `json:"hostname"`
	OS                        string   `json:"os"`
	KernelVersion             string   `json:"kernel_version"`
	IPAddress                 string   `json:"ip_address"`
	UptimeSeconds             uint64   `json:"uptime_seconds"`
	Load1                     float64  `json:"load_1"`
	Load5                     float64  `json:"load_5"`
	Load15                    float64  `json:"load_15"`
	ProcessCount              int      `json:"process_count"`
	HardwareTemperatureCelsius *float64 `json:"hardware_temperature_celsius"` // null quando não suportado
}

type ServerMetricsSnapshot struct {
	Timestamp time.Time          `json:"timestamp"`
	CPU       CPUMetrics         `json:"cpu"`
	Memory    MemoryMetrics      `json:"memory"`
	Disk      DiskActivity       `json:"disk"`
	Network   []NetworkInterface `json:"network"`
	System    SystemOverview     `json:"system"`
}
