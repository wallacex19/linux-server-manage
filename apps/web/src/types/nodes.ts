export type NodeStatus =
  | 'online'
  | 'offline'
  | 'connecting'
  | 'unpaired'
  | 'revoked'
  | 'error'

export interface NodeMetricsSummary {
  cpu_percent: number
  memory_percent: number
  disk_percent: number
  process_count: number
  uptime_seconds: number
  game_servers_count: number
  collected_at: string
  is_stale: boolean
}

export interface Node {
  id: string
  name: string
  hostname: string
  ip_address: string
  status: NodeStatus
  group_name: string
  tags: string
  agent_version?: string
  pairing_token?: string
  pairing_expires_at?: string
  revoked_at?: string
  last_heartbeat?: string
  last_error?: string
  os_info?: string
  install_command?: string
  created_at: string
  updated_at: string
  metrics?: NodeMetricsSummary
}

export interface CreateNodeRequest {
  name: string
  group_name: string
  tags: string
}
