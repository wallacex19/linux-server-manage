export interface PersistentSession {
  name: string
  id: string
  windows_count: number
  created_at: string
  is_attached: boolean
  is_game_server: boolean
  game_name?: string
}

export interface TerminalTab {
  id: string
  title: string
  tmuxSession?: string
}
