import { parseEditorType } from '@/domain/type-codec'
import type { ConfigEnumValue } from './queries'

export type EditorModelField = {
  key: string
  name: string
  description: string
  isArray: boolean
  type: string
}

export function requireModelFields(fields: EditorModelField[]): void {
  if (!Array.isArray(fields) || fields.length === 0) throw new Error('模型至少需要一个字段')
  const keys = new Set<string>()
  fields.forEach((field, index) => {
    if (!/^[A-Za-z][A-Za-z0-9]*$/.test(field.key)) throw new Error(`第 ${index + 1} 个字段的标识不合法`)
    if (!field.name) throw new Error(`第 ${index + 1} 个字段缺少名称`)
    if (keys.has(field.key)) throw new Error(`字段标识重复: ${field.key}`)
    keys.add(field.key)
    parseEditorType(field.type)
  })
}

export function requireEnumValues(values: ConfigEnumValue[]): void {
  if (!Array.isArray(values) || values.length === 0) throw new Error('枚举至少需要一个值')
  const keys = new Set<string>()
  values.forEach((item, index) => {
    if (!item.label) throw new Error(`第 ${index + 1} 个枚举值缺少显示文本`)
    if (!/^[A-Z][A-Z0-9_]*$/.test(item.value)) throw new Error(`第 ${index + 1} 个枚举值必须使用大写字母、数字或下划线`)
    if (keys.has(item.value)) throw new Error(`枚举值重复: ${item.value}`)
    keys.add(item.value)
  })
}
