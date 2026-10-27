export interface LinuxUserIdentity {
  username: string
  uid: number
  gid: number
  groups: string[]
  home_dir: string
  shell: string
  is_admin: boolean
}

export interface SystemInfo {
  hostname: string
  os: string
  architecture: string
  cpu_cores: number
  go_version: string
  uptime_seconds: number
}

export interface HealthResponse {
  status: string
  timestamp: string
  environment: string
  database: string
  version: string
}
