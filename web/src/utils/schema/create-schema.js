import { wrapSchema } from './wrapper.js'

export const baseTypeOptions = Object.freeze([
  { label: '字符串', value: '{"baseType":"String"}' },
  { label: '整数', value: '{"baseType":"Int"}' },
  { label: '小数', value: '{"baseType":"Decimal"}' },
  { label: '布尔', value: '{"baseType":"Boolean"}' },
  { label: '日期', value: '{"baseType":"Date"}' },
  { label: '时间', value: '{"baseType":"Time"}' },
  { label: '日期时间', value: '{"baseType":"Datetime"}' },
  { label: '日期范围', value: '{"baseType":"DateRange"}' },
  { label: '时间范围', value: '{"baseType":"TimeRange"}' },
  { label: '日期时间范围', value: '{"baseType":"DatetimeRange"}' },
  { label: '图片', value: '{"baseType":"Image"}' },
  { label: '视频', value: '{"baseType":"Video"}' },
  { label: '文件', value: '{"baseType":"File"}' },
])

const RANGE_TYPES = new Set(['DateRange', 'TimeRange', 'DatetimeRange'])
const FILE_TYPES = new Set(['Image', 'Video', 'File'])

const validateResources = (resources) => {
  if (!resources || !Array.isArray(resources.models) || !Array.isArray(resources.enums)) {
    throw new TypeError('createSchema 需要显式传入 { models, enums }')
  }
  return resources
}

const getNodeValue = (node) => {
  if (!node || typeof node !== 'object' || !node.value || typeof node.value !== 'object') {
    throw new TypeError('Schema 节点缺少 value')
  }
  if (!node.value.key || !node.value.type || typeof node.value.type !== 'object') {
    throw new TypeError('Schema 节点缺少 key 或 type')
  }
  return node.value
}

const fieldToNode = (field) => {
  if (field?.value) {
    return field
  }
  if (!field || typeof field !== 'object') {
    throw new TypeError('模型字段必须是对象')
  }
  return {
    value: {
      key: field.key,
      name: field.name,
      description: field.description,
      isArray: Boolean(field.isArray),
      type: field.type,
    },
    children: field.children || [],
  }
}

const findModel = (models, key) => models.find((model) => model.key === key)

const getStructureChildren = (node, structureKey, models) => {
  if (Array.isArray(node.children) && node.children.length > 0) {
    return node.children
  }
  const model = findModel(models, structureKey)
  if (!model) {
    throw new Error(`找不到模型: ${structureKey}`)
  }
  const fields = model.fields || model.children
  if (!Array.isArray(fields)) {
    throw new TypeError(`模型 ${structureKey} 缺少 fields`)
  }
  return fields.map(fieldToNode)
}

const withTooltip = (description) => {
  if (!description) {
    return {}
  }
  return {
    tooltip: description,
    tooltipLayout: 'text',
  }
}

const createEnumSchema = (title, description, options) => ({
  title,
  description,
  type: 'string',
  'x-decorator': 'FormItem',
  'x-component': 'Select',
  'x-component-props': { size: 'small' },
  enum: options,
})

export const createBaseTypeSchema = (
  baseType,
  title = '',
  description = '',
  needDecorator = false,
) => {
  const schema = {
    title,
    'x-decorator-props': withTooltip(description),
  }

  switch (baseType) {
    case 'Int':
      schema.type = 'number'
      schema['x-component'] = 'InputNumber'
      schema['x-component-props'] = { size: 'small', precision: 0 }
      schema['x-decorator-props'].wrapperStyle = 'width: 200px;'
      break
    case 'Decimal':
      schema.type = 'number'
      schema['x-component'] = 'InputNumber'
      schema['x-component-props'] = { size: 'small' }
      schema['x-decorator-props'].wrapperStyle = 'width: 200px;'
      break
    case 'Boolean':
      schema.type = 'boolean'
      schema['x-component'] = 'Switch'
      schema['x-component-props'] = { size: 'small' }
      break
    case 'Date':
      schema.type = 'string'
      schema['x-component'] = 'DatePicker'
      schema['x-component-props'] = {
        size: 'small',
        valueFormat: 'yyyy-MM-dd',
      }
      schema['x-decorator-props'].wrapperStyle = 'width: 200px;'
      break
    case 'Time':
      schema.type = 'string'
      schema['x-component'] = 'TimePicker'
      schema['x-component-props'] = {
        size: 'small',
        valueFormat: 'HH:mm:ss',
      }
      schema['x-decorator-props'].wrapperStyle = 'width: 200px;'
      break
    case 'Datetime':
      schema.type = 'string'
      schema['x-component'] = 'DatePicker'
      schema['x-component-props'] = {
        type: 'datetime',
        size: 'small',
        valueFormat: 'yyyy-MM-dd HH:mm:ss',
      }
      schema['x-decorator-props'].wrapperStyle = 'width: 200px;'
      break
    case 'DateRange':
      schema.type = 'array'
      schema['x-component'] = 'DatePicker'
      schema['x-component-props'] = {
        type: 'daterange',
        size: 'small',
        valueFormat: 'yyyy-MM-dd',
      }
      schema['x-decorator-props'].wrapperStyle = 'width: 500px;'
      break
    case 'TimeRange':
      schema.type = 'array'
      schema['x-component'] = 'TimePicker'
      schema['x-component-props'] = {
        isRange: true,
        size: 'small',
        valueFormat: 'HH:mm:ss',
      }
      schema['x-decorator-props'].wrapperStyle = 'width: 500px;'
      break
    case 'DatetimeRange':
      schema.type = 'array'
      schema['x-component'] = 'DatePicker'
      schema['x-component-props'] = {
        type: 'datetimerange',
        size: 'small',
        valueFormat: 'yyyy-MM-dd HH:mm:ss',
      }
      schema['x-decorator-props'].wrapperStyle = 'width: 500px;'
      break
    case 'Image':
    case 'Video':
    case 'File':
      schema.type = 'string'
      schema['x-component'] = 'FileUpload'
      schema['x-component-props'] = { uploadType: baseType, isArray: false }
      break
    case 'String':
    default:
      schema.type = 'string'
      schema['x-component'] = 'Input'
      schema['x-component-props'] = { size: 'small' }
  }

  if (needDecorator) {
    schema['x-decorator'] = 'FormItem'
  }
  return schema
}

const createObjectProperties = (children, resources, state) => {
  const properties = {}
  children.forEach((child) => {
    Object.assign(properties, createNodeSchema(child, resources, state))
  })
  return properties
}

const createObjectArraySchema = (value, properties) => ({
  type: 'array',
  title: value.name,
  'x-decorator': 'FormItem',
  'x-decorator-props': withTooltip(value.description),
  'x-component': 'ArrayCards',
  items: {
    type: 'object',
    properties: {
      index: { type: 'void', 'x-component': 'ArrayCards.Index' },
      ...properties,
      remove: { type: 'void', 'x-component': 'ArrayCards.Remove' },
      moveUp: { type: 'void', 'x-component': 'ArrayCards.MoveUp' },
      moveDown: { type: 'void', 'x-component': 'ArrayCards.MoveDown' },
    },
  },
  properties: {
    add: {
      type: 'void',
      title: '添加条目',
      'x-component': 'ArrayItems.Addition',
    },
  },
})

const createBaseArraySchema = (value, baseType) => {
  if (FILE_TYPES.has(baseType)) {
    return {
      type: 'array',
      title: value.name,
      'x-decorator': 'FormItem',
      'x-decorator-props': withTooltip(value.description),
      'x-component': 'FileUpload',
      'x-component-props': { uploadType: baseType, isArray: true },
    }
  }

  return {
    type: 'array',
    title: value.name,
    'x-component': 'ArrayItems',
    'x-decorator': 'FormItem',
    'x-decorator-props': withTooltip(value.description),
    items: {
      type: 'void',
      'x-component': 'Space',
      'x-decorator': 'FormItem',
      properties: {
        sort: { type: 'void', 'x-component': 'ArrayItems.SortHandle' },
        input: createBaseTypeSchema(baseType),
        remove: { type: 'void', 'x-component': 'ArrayItems.Remove' },
      },
    },
    properties: {
      add: {
        type: 'void',
        title: '添加条目',
        'x-component': 'ArrayItems.Addition',
        'x-component-props': { style: { marginTop: '10px' } },
      },
    },
  }
}

const createNodeSchema = (node, resources, state) => {
  const value = getNodeValue(node)
  const { baseType, enumKey, structureKey } = value.type

  if (structureKey) {
    if (state.modelPath.includes(structureKey)) {
      throw new Error(`检测到递归模型: ${[...state.modelPath, structureKey].join(' -> ')}`)
    }
    const children = getStructureChildren(node, structureKey, resources.models)
    const properties = createObjectProperties(children, resources, {
      modelPath: [...state.modelPath, structureKey],
    })
    return {
      [value.key]: value.isArray
        ? createObjectArraySchema(value, properties)
        : {
            type: 'object',
            title: value.name,
            'x-decorator': 'FormItem',
            'x-decorator-props': withTooltip(value.description),
            properties,
          },
    }
  }

  if (enumKey) {
    const enumConfig = resources.enums.find((item) => item.key === enumKey)
    if (!enumConfig) {
      throw new Error(`找不到枚举: ${enumKey}`)
    }
    const options = (enumConfig.values || []).map((item) => ({
      label: item.label,
      value: item.value,
    }))
    if (value.isArray) {
      return {
        [value.key]: {
          type: 'array',
          title: value.name,
          'x-decorator': 'FormItem',
          'x-component': 'ArrayItems',
          'x-decorator-props': withTooltip(value.description),
          items: createEnumSchema('', '', options),
          properties: {
            add: {
              type: 'void',
              title: '添加条目',
              'x-component': 'ArrayItems.Addition',
            },
          },
        },
      }
    }
    return {
      [value.key]: createEnumSchema(value.name, value.description, options),
    }
  }

  if (!baseType) {
    throw new TypeError(`Schema 节点 ${value.key} 没有可用类型`)
  }
  return {
    [value.key]: value.isArray
      ? createBaseArraySchema(value, baseType)
      : createBaseTypeSchema(baseType, value.name, value.description, true),
  }
}

export const createSchema = (node, resources) => {
  const checkedResources = validateResources(resources)
  return createNodeSchema(node, checkedResources, { modelPath: [] })
}

export const createWrappedSchema = (node, resources) => {
  const schema = createSchema(node, resources)
  return wrapSchema(schema, getNodeValue(node))
}

export const isRangeType = (baseType) => RANGE_TYPES.has(baseType)
