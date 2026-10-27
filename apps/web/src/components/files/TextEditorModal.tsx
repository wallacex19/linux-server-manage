import { useState, useEffect } from 'react'
import { Save, X, AlertTriangle, FileText, Check } from 'lucide-react'
import { FileContent } from '../../types/files'

interface TextEditorModalProps {
  filePath: string
  onClose: () => void
  onSaveSuccess?: () => void
}

export function TextEditorModal({
  filePath,
  onClose,
  onSaveSuccess,
}: TextEditorModalProps) {
  const [content, setContent] = useState('')
  const [originalContent, setOriginalContent] = useState('')
  const [isLoading, setIsLoading] = useState(true)
  const [isSaving, setIsSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [fileDetails, setFileDetails] = useState<FileContent | null>(null)

  const isDirty = content !== originalContent

  useEffect(() => {
    async function loadContent() {
      setIsLoading(true)
      setError(null)
      try {
        const res = await fetch(
          `/api/v1/files/read?path=${encodeURIComponent(filePath)}`,
          { credentials: 'include' },
        )
        if (!res.ok) {
          const errData = await res.json().catch(() => ({}))
          throw new Error(errData.error || 'Falha ao carregar conteúdo do arquivo')
        }
        const data: FileContent = await res.json()
        setContent(data.content)
        setOriginalContent(data.content)
        setFileDetails(data)
      } catch (err: unknown) {
        if (err instanceof Error) {
          setError(err.message)
        } else {
          setError('Erro inesperado')
        }
      } finally {
        setIsLoading(false)
      }
    }

    loadContent()
  }, [filePath])

  const handleSave = async () => {
    setIsSaving(true)
    setError(null)
    setSaveSuccess(false)
    try {
      const res = await fetch('/api/v1/files/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ path: filePath, content }),
      })
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}))
        throw new Error(errData.error || 'Falha ao salvar arquivo')
      }
      setOriginalContent(content)
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2500)
      if (onSaveSuccess) onSaveSuccess()
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message)
      } else {
        setError('Erro ao salvar')
      }
    } finally {
      setIsSaving(false)
    }
  }

  // Ctrl+S shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault()
        if (isDirty && !isSaving) {
          handleSave()
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [content, originalContent, isSaving])

  const handleClose = () => {
    if (isDirty) {
      if (
        !window.confirm(
          'Existem alterações não salvas neste arquivo. Deseja realmente fechar e descartar as mudanças?',
        )
      ) {
        return
      }
    }
    onClose()
  }

  const lines = content.split('\n')

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-5xl h-[85vh] flex flex-col shadow-2xl overflow-hidden">
        {/* Editor Top Bar */}
        <div className="h-14 px-5 border-b border-slate-800 bg-slate-900 flex items-center justify-between shrink-0">
          <div className="flex items-center gap-3 overflow-hidden">
            <FileText className="w-5 h-5 text-indigo-400 shrink-0" />
            <div className="truncate">
              <span className="text-sm font-semibold text-white truncate block">
                {filePath}
              </span>
              {fileDetails && (
                <span className="text-[11px] text-slate-400">
                  Permissões: {fileDetails.permissions} · Linhas: {lines.length}
                </span>
              )}
            </div>
            {isDirty && (
              <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
                Não salvo
              </span>
            )}
          </div>

          <div className="flex items-center gap-2">
            {saveSuccess && (
              <span className="flex items-center gap-1 text-xs text-emerald-400 mr-2">
                <Check className="w-3.5 h-3.5" /> Salvo!
              </span>
            )}
            <button
              onClick={handleSave}
              disabled={isSaving || !isDirty}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-indigo-600 hover:bg-indigo-500 disabled:bg-slate-800 disabled:text-slate-500 text-white transition-colors shadow-sm"
              title="Salvar arquivo (Ctrl+S)"
            >
              <Save className="w-3.5 h-3.5" />
              <span>{isSaving ? 'Gravando...' : 'Salvar (Ctrl+S)'}</span>
            </button>
            <button
              onClick={handleClose}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
              title="Fechar editor"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Error Banner */}
        {error && (
          <div className="p-3 bg-rose-500/10 border-b border-rose-500/20 text-xs text-rose-300 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-rose-400 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        {/* Main Editor Body */}
        {isLoading ? (
          <div className="flex-1 flex items-center justify-center text-xs text-slate-400">
            Carregando arquivo...
          </div>
        ) : (
          <div className="flex-1 flex bg-slate-950 font-mono text-xs overflow-hidden">
            {/* Line Numbers */}
            <div className="py-4 pl-3 pr-2 select-none text-slate-600 text-right bg-slate-950/60 border-r border-slate-800/80 w-12 overflow-hidden shrink-0">
              {lines.map((_, i) => (
                <div key={i} className="leading-5">
                  {i + 1}
                </div>
              ))}
            </div>

            {/* Code Textarea */}
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              className="flex-1 p-4 bg-transparent text-slate-200 outline-none resize-none leading-5 scrollbar-thin whitespace-pre font-mono"
              placeholder="Arquivo vazio..."
              spellCheck={false}
            />
          </div>
        )}
      </div>
    </div>
  )
}
