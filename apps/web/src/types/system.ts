export interface ProcessInfo {
  pid: number
  user: string
  uid: number
  name: string
  command: string
  cpu_percent: number
  memory_bytes: number
  memory_percent: number
  state: string
}

export interface ServiceInfo {
  name: string
  description: string
  load_state: string
  active_state: string
  sub_state: string
  can_control: boolean
}
