import { useState, useRef, ChangeEvent } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Folder,
  FileText,
  FileCode,
  ArrowUp,
  Download,
  Trash2,
  Edit2,
  Plus,
  FolderPlus,
  Upload,
  RefreshCw,
  Search,
  AlertCircle,
  File,
} from 'lucide-react'
import { DirectoryListing, FileEntry } from '../types/files'
import { TextEditorModal } from '../components/files/TextEditorModal'

async function fetchDirectory(path: string): Promise<DirectoryListing> {
  const url = path ? `/api/v1/files/list?path=${encodeURIComponent(path)}` : '/api/v1/files/list'
  const res = await fetch(url, { credentials: 'include' })
  if (!res.ok) {
    const errData = await res.json().catch(() => ({}))
    throw new Error(errData.error || 'Falha ao listar diretório')
  }
  return res.json()
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

export function FileManagerView() {
  const queryClient = useQueryClient()
  const [currentPath, setCurrentPath] = useState<string>('')
  const [searchTerm, setSearchTerm] = useState('')
  const [editingFile, setEditingFile] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [actionSuccess, setActionSuccess] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data: listing, isLoading, error } = useQuery({
    queryKey: ['files', currentPath],
    queryFn: () => fetchDirectory(currentPath),
  })

  const refreshFiles = () => {
    queryClient.invalidateQueries({ queryKey: ['files', currentPath] })
  }

  const showNotification = (msg: string, isError = false) => {
    if (isError) {
      setActionError(msg)
      setTimeout(() => setActionError(null), 4000)
    } else {
      setActionSuccess(msg)
      setTimeout(() => setActionSuccess(null), 3000)
    }
  }

  const handleCreateFolder = async () => {
    const name = window.prompt('Nome da nova pasta:')
    if (!name || !name.trim()) return

    const newPath = currentPath ? `${currentPath}/${name.trim()}` : name.trim()
    try {
      const res = await fetch('/api/v1/files/mkdir', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ path: newPath }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao criar pasta')
      }
      showNotification('Pasta criada com sucesso.')
      refreshFiles()
    } catch (err: unknown) {
      showNotification(err instanceof Error ? err.message : 'Erro ao criar pasta', true)
    }
  }

  const handleCreateFile = async () => {
    const name = window.prompt('Nome do novo arquivo (ex: config.yaml):')
    if (!name || !name.trim()) return

    const newPath = currentPath ? `${currentPath}/${name.trim()}` : name.trim()
    try {
      const res = await fetch('/api/v1/files/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ path: newPath }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao criar arquivo')
      }
      showNotification('Arquivo criado com sucesso.')
      refreshFiles()
      setEditingFile(newPath)
    } catch (err: unknown) {
      showNotification(err instanceof Error ? err.message : 'Erro ao criar arquivo', true)
    }
  }

  const handleDelete = async (entry: FileEntry) => {
    const itemType = entry.is_dir ? 'a pasta' : 'o arquivo'
    if (
      !window.confirm(
        `Tem certeza que deseja excluir ${itemType} "${entry.name}"? Esta ação não pode ser desfeita.`,
      )
    ) {
      return
    }

    try {
      const res = await fetch(`/api/v1/files/delete?path=${encodeURIComponent(entry.path)}`, {
        method: 'DELETE',
        credentials: 'include',
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao excluir')
      }
      showNotification('Excluído com sucesso.')
      refreshFiles()
    } catch (err: unknown) {
      showNotification(err instanceof Error ? err.message : 'Erro ao excluir', true)
    }
  }

  const handleRename = async (entry: FileEntry) => {
    const newName = window.prompt('Novo nome:', entry.name)
    if (!newName || !newName.trim() || newName.trim() === entry.name) return

    const parentDir = entry.path.substring(0, entry.path.lastIndexOf(entry.name))
    const newPath = `${parentDir}${newName.trim()}`

    try {
      const res = await fetch('/api/v1/files/rename', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ old_path: entry.path, new_path: newPath }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao renomear')
      }
      showNotification('Renomeado com sucesso.')
      refreshFiles()
    } catch (err: unknown) {
      showNotification(err instanceof Error ? err.message : 'Erro ao renomear', true)
    }
  }

  const handleUpload = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return

    const formData = new FormData()
    formData.append('file', file)
    formData.append('target_dir', listing?.current_path || '')

    try {
      const res = await fetch('/api/v1/files/upload', {
        method: 'POST',
        credentials: 'include',
        body: formData,
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha no upload do arquivo')
      }
      showNotification(`Upload de "${file.name}" concluído com sucesso.`)
      refreshFiles()
    } catch (err: unknown) {
      showNotification(err instanceof Error ? err.message : 'Erro ao enviar arquivo', true)
    } finally {
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  const filteredEntries =
    listing?.entries.filter((entry) =>
      entry.name.toLowerCase().includes(searchTerm.toLowerCase()),
    ) || []

  return (
    <div className="space-y-4">
      {/* Top Breadcrumb & Control Bar */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
        {/* Breadcrumb Navigation */}
        <div className="flex items-center gap-2 overflow-x-auto text-xs py-1 scrollbar-thin">
          <button
            onClick={() => setCurrentPath('')}
            className="p-1 rounded bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 transition-colors shrink-0"
            title="Diretório Home"
          >
            Home
          </button>
          {listing?.parent_path && (
            <button
              onClick={() => setCurrentPath(listing.parent_path!)}
              className="p-1 rounded bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 transition-colors shrink-0 flex items-center gap-1"
              title="Subir um nível"
            >
              <ArrowUp className="w-3.5 h-3.5" />
              <span>Acima</span>
            </button>
          )}
          <span className="text-slate-500 font-mono text-xs truncate max-w-md">
            {listing?.current_path || 'Carregando...'}
          </span>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-2 shrink-0">
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleUpload}
            className="hidden"
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition-colors"
          >
            <Upload className="w-3.5 h-3.5" />
            <span>Enviar</span>
          </button>
          <button
            onClick={handleCreateFolder}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition-colors"
          >
            <FolderPlus className="w-3.5 h-3.5" />
            <span>Nova Pasta</span>
          </button>
          <button
            onClick={handleCreateFile}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-indigo-600 hover:bg-indigo-500 text-white transition-colors shadow-sm"
          >
            <Plus className="w-3.5 h-3.5" />
            <span>Novo Arquivo</span>
          </button>
          <button
            onClick={refreshFiles}
            className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white hover:bg-slate-700 transition-colors"
            title="Atualizar listagem"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Notification Toasts */}
      {actionError && (
        <div className="p-3 bg-rose-500/10 border border-rose-500/20 text-xs text-rose-300 rounded-lg flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
          <span>{actionError}</span>
        </div>
      )}
      {actionSuccess && (
        <div className="p-3 bg-emerald-500/10 border border-emerald-500/20 text-xs text-emerald-300 rounded-lg flex items-center gap-2">
          <span>{actionSuccess}</span>
        </div>
      )}

      {/* Search Input Filter */}
      <div className="relative">
        <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
        <input
          type="text"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          placeholder="Filtrar arquivos na pasta atual..."
          className="w-full pl-9 pr-4 py-2 bg-slate-900 border border-slate-800 rounded-xl text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500"
        />
      </div>

      {/* Files Table List */}
      <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
        {isLoading ? (
          <div className="p-10 text-center text-xs text-slate-400">
            Carregando arquivos do sistema...
          </div>
        ) : error ? (
          <div className="p-10 text-center text-xs text-rose-400">
            {error instanceof Error ? error.message : 'Falha ao carregar diretório'}
          </div>
        ) : filteredEntries.length === 0 ? (
          <div className="p-10 text-center text-xs text-slate-500">
            Nenhum arquivo ou pasta encontrada neste local.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-slate-800 text-slate-400 bg-slate-950/40">
                  <th className="py-2.5 px-4 font-medium">Nome</th>
                  <th className="py-2.5 px-4 font-medium">Tamanho</th>
                  <th className="py-2.5 px-4 font-medium">Permissões</th>
                  <th className="py-2.5 px-4 font-medium">Última Modificação</th>
                  <th className="py-2.5 px-4 font-medium text-right">Ações</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-300">
                {filteredEntries.map((entry) => (
                  <tr
                    key={entry.path}
                    className="hover:bg-slate-800/40 transition-colors group"
                  >
                    <td className="py-2.5 px-4">
                      {entry.is_dir ? (
                        <button
                          onClick={() => setCurrentPath(entry.path)}
                          className="flex items-center gap-2.5 font-medium text-indigo-400 hover:text-indigo-300 transition-colors text-left"
                        >
                          <Folder className="w-4 h-4 text-indigo-400 shrink-0" />
                          <span className="truncate">{entry.name}</span>
                        </button>
                      ) : (
                        <button
                          onClick={() => {
                            if (!entry.is_binary) {
                              setEditingFile(entry.path)
                            }
                          }}
                          className={`flex items-center gap-2.5 text-left ${
                            entry.is_binary
                              ? 'text-slate-400 cursor-default'
                              : 'text-slate-200 hover:text-white hover:underline'
                          }`}
                        >
                          {entry.name.endsWith('.yaml') || entry.name.endsWith('.json') || entry.name.endsWith('.sh') ? (
                            <FileCode className="w-4 h-4 text-emerald-400 shrink-0" />
                          ) : entry.is_binary ? (
                            <File className="w-4 h-4 text-slate-500 shrink-0" />
                          ) : (
                            <FileText className="w-4 h-4 text-indigo-400 shrink-0" />
                          )}
                          <span className="truncate">{entry.name}</span>
                          {entry.is_binary && (
                            <span className="text-[10px] text-slate-500 font-mono ml-1">
                              [binário]
                            </span>
                          )}
                        </button>
                      )}
                    </td>
                    <td className="py-2.5 px-4 text-slate-400">
                      {entry.is_dir ? '—' : formatBytes(entry.size)}
                    </td>
                    <td className="py-2.5 px-4 font-mono text-slate-400 text-[11px]">
                      {entry.permissions}
                    </td>
                    <td className="py-2.5 px-4 text-slate-400">
                      {new Date(entry.mod_time).toLocaleString([], {
                        dateStyle: 'short',
                        timeStyle: 'short',
                      })}
                    </td>
                    <td className="py-2.5 px-4 text-right">
                      <div className="flex items-center justify-end gap-1.5 opacity-90 group-hover:opacity-100">
                        {!entry.is_dir && !entry.is_binary && (
                          <button
                            onClick={() => setEditingFile(entry.path)}
                            className="p-1.5 rounded hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
                            title="Editar arquivo"
                          >
                            <Edit2 className="w-3.5 h-3.5" />
                          </button>
                        )}
                        {!entry.is_dir && (
                          <a
                            href={`/api/v1/files/download?path=${encodeURIComponent(entry.path)}`}
                            download
                            className="p-1.5 rounded hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
                            title="Baixar arquivo"
                          >
                            <Download className="w-3.5 h-3.5" />
                          </a>
                        )}
                        <button
                          onClick={() => handleRename(entry)}
                          className="p-1.5 rounded hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
                          title="Renomear"
                        >
                          <Edit2 className="w-3.5 h-3.5 text-slate-400" />
                        </button>
                        <button
                          onClick={() => handleDelete(entry)}
                          className="p-1.5 rounded hover:bg-rose-950/60 text-slate-400 hover:text-rose-400 transition-colors"
                          title="Excluir"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Text Editor Modal */}
      {editingFile && (
        <TextEditorModal
          filePath={editingFile}
          onClose={() => setEditingFile(null)}
          onSaveSuccess={refreshFiles}
        />
      )}
    </div>
  )
}
