import type {
  ModelField,
  SchemaNode,
  SchemaResources,
} from './types'

const DATE_RE = /^\d{4}-\d{2}-\d{2}$/
const TIME_RE = /^\d{2}:\d{2}:\d{2}$/
const DATETIME_RE = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/
const BASE_TYPES = new Set([
  'String',
  'Int',
  'Decimal',
  'Boolean',
  'Date',
  'Time',
  'Datetime',
  'DateRange',
  'TimeRange',
  'DatetimeRange',
  'Image',
  'Video',
  'File',
])

function assertBaseType(baseType: string): void {
  if (!BASE_TYPES.has(baseType)) {
    throw new Error(`不支持的基本类型: ${baseType}`)
  }
}

function getBaseTypeDefault(baseType: string): unknown {
  assertBaseType(baseType)
  if (baseType === 'Int' || baseType === 'Decimal') {
    return 0
  }
  if (baseType === 'Boolean') {
    return false
  }
  if (
    baseType === 'DateRange' ||
    baseType === 'TimeRange' ||
    baseType === 'DatetimeRange'
  ) {
    return []
  }
  return ''
}

function isValidRange(value: unknown, expression: RegExp): boolean {
  return (
    Array.isArray(value) &&
    value.length === 2 &&
    value.every(
      (item) => typeof item === 'string' && expression.test(item),
    )
  )
}

function isValidBaseType(baseType: string, value: unknown): boolean {
  assertBaseType(baseType)
  switch (baseType) {
    case 'Int':
      return Number.isInteger(value)
    case 'Decimal':
      return typeof value === 'number' && Number.isFinite(value)
    case 'Boolean':
      return typeof value === 'boolean'
    case 'Date':
      return typeof value === 'string' && DATE_RE.test(value)
    case 'Time':
      return typeof value === 'string' && TIME_RE.test(value)
    case 'Datetime':
      return typeof value === 'string' && DATETIME_RE.test(value)
    case 'DateRange':
      return isValidRange(value, DATE_RE)
    case 'TimeRange':
      return isValidRange(value, TIME_RE)
    case 'DatetimeRange':
      return isValidRange(value, DATETIME_RE)
    case 'String':
    case 'Image':
    case 'Video':
    case 'File':
      return typeof value === 'string'
    default:
      throw new Error(`不支持的基本类型: ${baseType}`)
  }
}

function validateResources(resources: SchemaResources): SchemaResources {
  if (
    !resources ||
    !Array.isArray(resources.models) ||
    !Array.isArray(resources.enums)
  ) {
    throw new TypeError('normalizeData 需要显式传入 { models, enums }')
  }
  return resources
}

function fieldToNode(field: ModelField | SchemaNode): SchemaNode {
  if ('value' in field) {
    return field
  }
  return {
    value: {
      key: field.key,
      name: field.name ?? field.key,
      description: field.description ?? '',
      isArray: Boolean(field.isArray),
      type: field.type,
    },
    children: field.children ?? [],
  }
}

function getChildren(
  node: SchemaNode,
  structureKey: string,
  resources: SchemaResources,
): SchemaNode[] {
  if (node.children && node.children.length > 0) {
    return node.children
  }
  const model = resources.models.find((item) => item.key === structureKey)
  if (!model) {
    throw new Error(`找不到模型: ${structureKey}`)
  }
  const fields = model.fields ?? model.children
  if (!Array.isArray(fields)) {
    throw new TypeError(`模型 ${structureKey} 缺少 fields`)
  }
  return fields.map(fieldToNode)
}

function asRecord(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

function normalizeObject(
  node: SchemaNode,
  data: unknown,
  resources: SchemaResources,
  modelPath: string[],
  structureKey: string,
): Record<string, unknown> {
  if (modelPath.includes(structureKey)) {
    throw new Error(
      `检测到递归模型: ${[...modelPath, structureKey].join(' -> ')}`,
    )
  }
  const source = asRecord(data)
  const result: Record<string, unknown> = {}
  const children = getChildren(node, structureKey, resources)
  const nextPath = [...modelPath, structureKey]
  children.forEach((child) => {
    result[child.value.key] = normalizeValue(
      child,
      source[child.value.key],
      resources,
      nextPath,
    )
  })
  return result
}

function normalizeValue(
  node: SchemaNode,
  data: unknown,
  resources: SchemaResources,
  modelPath: string[],
): unknown {
  const { baseType, enumKey, structureKey } = node.value.type

  if (structureKey) {
    if (node.value.isArray) {
      return Array.isArray(data)
        ? data.map((item) =>
            normalizeObject(node, item, resources, modelPath, structureKey),
          )
        : []
    }
    return normalizeObject(node, data, resources, modelPath, structureKey)
  }

  if (enumKey) {
    if (!resources.enums.some((item) => item.key === enumKey)) {
      throw new Error(`找不到枚举: ${enumKey}`)
    }
    if (node.value.isArray) {
      return Array.isArray(data)
        ? data.map((item) => (typeof item === 'string' ? item : ''))
        : []
    }
    return typeof data === 'string' ? data : ''
  }

  if (!baseType) {
    return data
  }
  assertBaseType(baseType)
  if (node.value.isArray) {
    return Array.isArray(data)
      ? data.map((item) =>
          isValidBaseType(baseType, item)
            ? item
            : getBaseTypeDefault(baseType),
        )
      : []
  }
  return isValidBaseType(baseType, data)
    ? data
    : getBaseTypeDefault(baseType)
}

export function normalizeData(
  node: SchemaNode,
  data: unknown,
  resources: SchemaResources,
): unknown {
  const checkedResources = validateResources(resources)
  if (data === null || data === undefined || data === '') {
    return normalizeValue(node, undefined, checkedResources, [])
  }
  if (typeof data !== 'string') {
    throw new TypeError('normalizeData 的 data 必须是 JSON 字符串')
  }

  let parsed: unknown
  try {
    parsed = JSON.parse(data) as unknown
  } catch (error: unknown) {
    const message = error instanceof Error ? error.message : String(error)
    throw new SyntaxError(
      `配置项 ${node.value.key} 的值不是合法 JSON: ${message}`,
      { cause: error },
    )
  }
  return normalizeValue(node, parsed, checkedResources, [])
}
