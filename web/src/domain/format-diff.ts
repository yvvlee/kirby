import { t } from '@/i18n'

export function formatDiffValue(value: unknown): string {
  const serialized = JSON.stringify(value === undefined ? null : value, null, 2)
  if (typeof serialized !== 'string') {
    throw new TypeError(t('差异对比值无法转换为 JSON'))
  }
  return serialized
}
