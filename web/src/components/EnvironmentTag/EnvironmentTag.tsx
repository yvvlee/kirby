import type { Environment } from '@/api/types'
import { environmentTagStyle } from './environment-tag-style'
import { useTranslation } from '@/i18n'

export default function EnvironmentTag({ environment }: { environment: Environment | null }) {
  const { t } = useTranslation()
  const label = environment?.name ?? t('未选择环境')
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
