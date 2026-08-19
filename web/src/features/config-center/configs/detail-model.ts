export function formatConfigJSON(value: string): string {
  if (value === '') return ''
  if (typeof value !== 'string') throw new TypeError('配置值必须是 JSON 字符串')
  return JSON.stringify(JSON.parse(value), null, 2)
}
