import type { Environment } from '@/api/types'
import { environmentTagStyle } from './environment-tag-style'

export default function EnvironmentTag({ environment }: { environment: Environment | null }) {
  const label = environment?.name ?? '未选择环境'
  return (
    <span
      className="environment-tag"
      style={environmentTagStyle(environment)}
      title={environment?.description ?? label}
    >
      {label}
    </span>
  )
}
