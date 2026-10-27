export interface SteamCMDStatus {
  installed: boolean
  path: string
  version: string
  libs_32bit_ready: boolean
  os_distribution: string
  install_guidance: string
}

export interface PopularGamePreset {
  name: string
  app_id: string
  service_type: string
  default_port: number
  default_args: string
  anonymous: boolean
  description: string
}

export interface GameServer {
  id: string
  node_id: string
  name: string
  service_type: string
  app_id: string
  install_path: string
  system_user: string
  session_name: string
  status: 'running' | 'stopped' | 'updating' | 'error'
  auto_restart: boolean
  start_command: string
  port: number
  created_at: string
  updated_at: string
}

export interface CreateServerRequest {
  name: string
  service_type: string
  app_id: string
  install_path: string
  start_command: string
  port: number
  auto_restart: boolean
}
