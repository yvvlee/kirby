import { parseEditorType } from '@/domain/type-codec'
import { localizedError } from '@/i18n/errors'
import type { ConfigEnumValue } from './queries'

export type EditorModelField = {
  key: string
  name: string
  description: string
  isArray: boolean
  type: string
}

export function requireModelFields(fields: EditorModelField[]): void {
  if (!Array.isArray(fields) || fields.length === 0) throw localizedError('模型至少需要一个字段')
  const keys = new Set<string>()
  fields.forEach((field, index) => {
    if (!/^[A-Za-z][A-Za-z0-9]*$/.test(field.key)) throw localizedError('第 {{index}} 个字段的标识不合法', { index: index + 1 })
    if (!field.name) throw localizedError('第 {{index}} 个字段缺少名称', { index: index + 1 })
    if (keys.has(field.key)) throw localizedError('字段标识重复: {{key}}', { key: field.key })
    keys.add(field.key)
    parseEditorType(field.type)
  })
}

export function requireEnumValues(values: ConfigEnumValue[]): void {
  if (!Array.isArray(values) || values.length === 0) throw localizedError('枚举至少需要一个值')
  const keys = new Set<string>()
  values.forEach((item, index) => {
    if (!item.label) throw localizedError('第 {{index}} 个枚举值缺少显示文本', { index: index + 1 })
    if (!/^[A-Z][A-Z0-9_]*$/.test(item.value)) throw localizedError('第 {{index}} 个枚举值必须使用大写字母、数字或下划线', { index: index + 1 })
    if (keys.has(item.value)) throw localizedError('枚举值重复: {{value}}', { value: item.value })
    keys.add(item.value)
  })
}
