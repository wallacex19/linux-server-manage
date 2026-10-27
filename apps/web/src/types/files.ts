export interface FileEntry {
  name: string
  path: string
  is_dir: boolean
  is_symlink: boolean
  size: number
  permissions: string
  mod_time: string
  mime_type: string
  is_binary: boolean
}

export interface DirectoryListing {
  current_path: string
  parent_path?: string
  entries: FileEntry[]
  total_items: number
}

export interface FileContent {
  path: string
  content: string
  size: number
  permissions: string
  mod_time: string
  is_binary: boolean
}
