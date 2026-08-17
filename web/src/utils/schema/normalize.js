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

const assertBaseType = (baseType) => {
  if (!BASE_TYPES.has(baseType)) {
    throw new Error(`不支持的基本类型: ${baseType}`)
  }
}

const getBaseTypeDefault = (baseType) => {
  assertBaseType(baseType)
  if (baseType === 'Int' || baseType === 'Decimal') {
    return 0
  }
  if (baseType === 'Boolean') {
    return false
  }
  if (baseType === 'DateRange' || baseType === 'TimeRange' || baseType === 'DatetimeRange') {
    return []
  }
  return ''
}

const isValidRange = (value, expression) =>
  Array.isArray(value) &&
  value.length === 2 &&
  value.every((item) => typeof item === 'string' && expression.test(item))

const isValidBaseType = (baseType, value) => {
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

const validateResources = (resources) => {
  if (!resources || !Array.isArray(resources.models) || !Array.isArray(resources.enums)) {
    throw new TypeError('normalizeData 需要显式传入 { models, enums }')
  }
  return resources
}

const getNodeValue = (node) => {
  if (!node?.value?.key || !node.value.type) {
    throw new TypeError('Schema 节点缺少 key 或 type')
  }
  return node.value
}

const fieldToNode = (field) =>
  field?.value
    ? field
    : {
        value: {
          key: field.key,
          name: field.name,
          description: field.description,
          isArray: Boolean(field.isArray),
          type: field.type,
        },
        children: field.children || [],
      }

const getChildren = (node, structureKey, resources) => {
  if (Array.isArray(node.children) && node.children.length > 0) {
    return node.children
  }
  const model = resources.models.find((item) => item.key === structureKey)
  if (!model) {
    throw new Error(`找不到模型: ${structureKey}`)
  }
  const fields = model.fields || model.children
  if (!Array.isArray(fields)) {
    throw new TypeError(`模型 ${structureKey} 缺少 fields`)
  }
  return fields.map(fieldToNode)
}

const normalizeObject = (node, data, resources, modelPath) => {
  const structureKey = node.value.type.structureKey
  if (modelPath.includes(structureKey)) {
    throw new Error(`检测到递归模型: ${[...modelPath, structureKey].join(' -> ')}`)
  }
  const source = data && typeof data === 'object' && !Array.isArray(data) ? data : {}
  const result = {}
  const children = getChildren(node, structureKey, resources)
  const nextPath = [...modelPath, structureKey]
  children.forEach((child) => {
    const childNode = fieldToNode(child)
    result[childNode.value.key] = normalizeValue(
      childNode,
      source[childNode.value.key],
      resources,
      nextPath,
    )
  })
  return result
}

const normalizeValue = (node, data, resources, modelPath) => {
  const value = getNodeValue(node)
  const { baseType, enumKey, structureKey } = value.type

  if (structureKey) {
    if (value.isArray) {
      if (!Array.isArray(data)) {
        return []
      }
      return data.map((item) => normalizeObject(node, item, resources, modelPath))
    }
    return normalizeObject(node, data, resources, modelPath)
  }

  if (enumKey) {
    if (!resources.enums.some((item) => item.key === enumKey)) {
      throw new Error(`找不到枚举: ${enumKey}`)
    }
    if (value.isArray) {
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
  if (value.isArray) {
    if (!Array.isArray(data)) {
      return []
    }
    return data.map((item) =>
      isValidBaseType(baseType, item) ? item : getBaseTypeDefault(baseType),
    )
  }
  return isValidBaseType(baseType, data) ? data : getBaseTypeDefault(baseType)
}

export const normalizeData = (node, data, resources) => {
  const checkedResources = validateResources(resources)
  if (data === null || data === undefined || data === '') {
    return normalizeValue(node, undefined, checkedResources, [])
  }
  if (typeof data !== 'string') {
    throw new TypeError('normalizeData 的 data 必须是 JSON 字符串')
  }

  let parsed
  try {
    parsed = JSON.parse(data)
  } catch (error) {
    throw new SyntaxError(
      `配置项 ${getNodeValue(node).key} 的值不是合法 JSON: ${error.message}`,
      { cause: error },
    )
  }
  return normalizeValue(node, parsed, checkedResources, [])
}
