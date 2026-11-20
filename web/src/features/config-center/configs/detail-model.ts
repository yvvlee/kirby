import { localizedError } from '@/i18n/errors'

export function formatConfigJSON(value: string): string {
  if (value === '') return ''
  if (typeof value !== 'string') throw localizedError('配置值必须是 JSON 字符串', {}, TypeError)
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch (error: unknown) {
    if (!(error instanceof SyntaxError)) throw error
    const localized = localizedError(
      '配置内容不是合法 JSON: {{message}}',
      { message: error.message },
      SyntaxError,
    )
    Object.defineProperty(localized, 'cause', { value: error })
    throw localized
  }
}
