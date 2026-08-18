const API_TO_EDITOR_BASE_TYPE = Object.freeze({
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
  Object.fromEntries(
    Object.entries(API_TO_EDITOR_BASE_TYPE).map(([api, editor]) => [
      editor,
      api,
    ]),
  ),
)

function requireType(type) {
  if (!type || typeof type !== 'object' || Array.isArray(type)) {
    throw new TypeError('字段类型必须是对象')
  }
  return type
}

export function toEditorType(type) {
  type = requireType(type)
  if (type.base_type) {
    const baseType = API_TO_EDITOR_BASE_TYPE[type.base_type]
    if (!baseType) {
      throw new Error(`不支持的 API 基本类型: ${type.base_type}`)
    }
    return { baseType }
  }
  if (type.structure_key) {
    return { structureKey: type.structure_key }
  }
  if (type.enum_key) {
    return { enumKey: type.enum_key }
  }
  throw new Error('字段类型没有 base_type、structure_key 或 enum_key')
}

export function toApiType(type) {
  type = requireType(type)
  if (type.baseType) {
    const baseType = EDITOR_TO_API_BASE_TYPE[type.baseType]
    if (!baseType) {
      throw new Error(`不支持的编辑器基本类型: ${type.baseType}`)
    }
    return { base_type: baseType }
  }
  if (type.structureKey) {
    return { structure_key: type.structureKey }
  }
  if (type.enumKey) {
    return { enum_key: type.enumKey }
  }
  throw new Error('字段类型没有 baseType、structureKey 或 enumKey')
}

export function normalizeField(field) {
  if (!field || typeof field !== 'object') {
    throw new TypeError('字段必须是对象')
  }
  return {
    ...field,
    isArray: Boolean(field.is_array),
    type: toEditorType(field.type),
  }
}

export function normalizeModel(model) {
  if (!model || typeof model !== 'object' || !Array.isArray(model.fields)) {
    throw new TypeError('模型响应缺少 fields')
  }
  return {
    ...model,
    fields: model.fields.map(normalizeField),
  }
}

export function normalizeTree(node) {
  if (!node || typeof node !== 'object' || !node.value) {
    throw new TypeError('配置结构响应缺少 value')
  }
  return {
    ...node,
    value: normalizeField(node.value),
    children: Array.isArray(node.children)
      ? node.children.map(normalizeTree)
      : [],
  }
}

export function parseEditorType(value) {
  if (typeof value !== 'string' || value.length === 0) {
    throw new TypeError('请选择字段类型')
  }
  let parsed
  try {
    parsed = JSON.parse(value)
  } catch (error) {
    throw new SyntaxError(`字段类型不是合法 JSON: ${error.message}`, {
      cause: error,
    })
  }
  return requireType(parsed)
}

export function stringifyEditorType(type) {
  return JSON.stringify(toEditorType(type))
}
