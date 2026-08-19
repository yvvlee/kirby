import type { ModelField, ModelResource, SchemaNode, SchemaValueType } from './schema'

const API_TO_EDITOR_BASE_TYPE: Readonly<Record<string, string>> = Object.freeze({
  BOOLEAN: 'Boolean',
  DATE: 'Date',
  DATETIME: 'Datetime',
  DATETIME_RANGE: 'DatetimeRange',
  DECIMAL: 'Decimal',
  FILE: 'File',
  IMAGE: 'Image',
  INT: 'Int',
  STRING: 'String',
  TIME: 'Time',
  TIME_RANGE: 'TimeRange',
  DATE_RANGE: 'DateRange',
  VIDEO: 'Video',
})

const EDITOR_TO_API_BASE_TYPE = Object.freeze(
  Object.fromEntries(Object.entries(API_TO_EDITOR_BASE_TYPE).map(([api, editor]) => [editor, api])),
)

type UnknownRecord = Record<string, unknown>

function requireType(type: unknown): UnknownRecord {
  if (typeof type !== 'object' || type === null || Array.isArray(type)) {
    throw new TypeError('字段类型必须是对象')
  }
  return type as UnknownRecord
}

export function toEditorType(type: unknown): SchemaValueType {
  const source = requireType(type)
  if (typeof source.base_type === 'string' && source.base_type) {
    const baseType = API_TO_EDITOR_BASE_TYPE[source.base_type]
    if (!baseType) throw new Error(`不支持的 API 基本类型: ${source.base_type}`)
    return { baseType }
  }
  if (typeof source.structure_key === 'string' && source.structure_key) return { structureKey: source.structure_key }
  if (typeof source.enum_key === 'string' && source.enum_key) return { enumKey: source.enum_key }
  throw new Error('字段类型没有 base_type、structure_key 或 enum_key')
}

export function toApiType(type: unknown): UnknownRecord {
  const source = requireType(type)
  if (typeof source.baseType === 'string' && source.baseType) {
    const baseType = EDITOR_TO_API_BASE_TYPE[source.baseType]
    if (!baseType) throw new Error(`不支持的编辑器基本类型: ${source.baseType}`)
    return { base_type: baseType }
  }
  if (typeof source.structureKey === 'string' && source.structureKey) return { structure_key: source.structureKey }
  if (typeof source.enumKey === 'string' && source.enumKey) return { enum_key: source.enumKey }
  throw new Error('字段类型没有 baseType、structureKey 或 enumKey')
}

export function normalizeField(field: unknown): ModelField {
  if (typeof field !== 'object' || field === null || Array.isArray(field)) throw new TypeError('字段必须是对象')
  const source = field as UnknownRecord
  if (typeof source.key !== 'string') throw new TypeError('字段缺少 key')
  return {
    key: source.key,
    name: typeof source.name === 'string' ? source.name : source.key,
    description: typeof source.description === 'string' ? source.description : '',
    isArray: Boolean(source.is_array),
    type: toEditorType(source.type),
  }
}

export function normalizeModel(model: unknown): ModelResource & UnknownRecord {
  if (typeof model !== 'object' || model === null || Array.isArray(model)) throw new TypeError('模型响应缺少 fields')
  const source = model as UnknownRecord
  if (typeof source.key !== 'string' || !Array.isArray(source.fields)) throw new TypeError('模型响应缺少 fields')
  return { ...source, key: source.key, fields: source.fields.map(normalizeField) }
}

export function normalizeTree(node: unknown): SchemaNode {
  if (typeof node !== 'object' || node === null || Array.isArray(node)) throw new TypeError('配置结构响应缺少 value')
  const source = node as UnknownRecord
  const value = normalizeField(source.value)
  return {
    value,
    children: Array.isArray(source.children) ? source.children.map(normalizeTree) : [],
  }
}

export function parseEditorType(value: string): SchemaValueType {
  if (typeof value !== 'string' || value.length === 0) throw new TypeError('请选择字段类型')
  try {
    return requireType(JSON.parse(value) as unknown) as SchemaValueType
  } catch (error: unknown) {
    if (error instanceof SyntaxError) throw new SyntaxError(`字段类型不是合法 JSON: ${error.message}`, { cause: error })
    throw error
  }
}

export function stringifyEditorType(type: unknown): string {
  return JSON.stringify(toEditorType(type))
}
