import { Clock, Shield, Sparkles } from 'lucide-react'

interface PlaceholderViewProps {
  title: string
  category: string
  description?: string
  roadmapStage?: string
}

export function PlaceholderView({
  title,
  category,
  description,
  roadmapStage,
}: PlaceholderViewProps) {
  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-8 max-w-3xl mx-auto space-y-6 text-center">
      <div className="inline-flex p-3 bg-indigo-500/10 border border-indigo-500/20 rounded-2xl text-indigo-400">
        <Sparkles className="w-8 h-8" />
      </div>

      <div className="space-y-2">
        <span className="text-xs font-semibold uppercase tracking-wider text-indigo-400">
          {category}
        </span>
        <h2 className="text-xl font-bold text-white">{title}</h2>
        <p className="text-xs text-slate-400 max-w-lg mx-auto leading-relaxed">
          {description ||
            'Este módulo está estruturado na arquitetura do Linux Server Manager e será ativado na fase subsequente correspondente.'}
        </p>
      </div>

      <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
        <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs bg-slate-800 text-slate-300 border border-slate-700">
          <Clock className="w-3.5 h-3.5 text-amber-400" />
          {roadmapStage || 'Etapa Roadmap LSM'}
        </span>
        <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs bg-slate-800 text-slate-300 border border-slate-700">
          <Shield className="w-3.5 h-3.5 text-emerald-400" />
          Isolamento de Permissões Nativo
        </span>
      </div>
    </div>
  )
}
