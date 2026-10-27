export interface CPUMetrics {
  total_percent: number
  per_core_percent?: number[]
  cores: number
}

export interface MemoryMetrics {
  total_bytes: number
  used_bytes: number
  available_bytes: number
  used_percent: number
  swap_total_bytes: number
  swap_used_bytes: number
  swap_used_percent: number
}

export interface DiskPartition {
  mount_point: string
  device: string
  fs_type: string
  total_bytes: number
  used_bytes: number
  free_bytes: number
  used_percent: number
}

export interface DiskActivity {
  read_bytes_per_sec: number
  write_bytes_per_sec: number
  partitions: DiskPartition[]
}

export interface NetworkInterface {
  name: string
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  rx_total_bytes: number
  tx_total_bytes: number
}

export interface SystemOverview {
  hostname: string
  os: string
  kernel_version: string
  ip_address: string
  uptime_seconds: number
  load_1: number
  load_5: number
  load_15: number
  process_count: number
  hardware_temperature_celsius: number | null
}

export interface ServerMetricsSnapshot {
  timestamp: string
  cpu: CPUMetrics
  memory: MemoryMetrics
  disk: DiskActivity
  network: NetworkInterface[]
  system: SystemOverview
}
