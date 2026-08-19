import { wrapSchema } from './wrapper'
import type {
  JsonSchema,
  ModelField,
  SchemaNode,
  SchemaResources,
  SchemaValueConfig,
} from './types'

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

function validateResources(
  resources?: SchemaResources,
): SchemaResources {
  if (
    !resources ||
    !Array.isArray(resources.models) ||
    !Array.isArray(resources.enums)
  ) {
    throw new TypeError('createSchema 需要显式传入 { models, enums }')
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

function getStructureChildren(
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

function withTooltip(description?: string): Record<string, unknown> {
  return description
    ? { tooltip: description, tooltipLayout: 'text' }
    : {}
}

function createEnumSchema(
  title: string,
  description: string | undefined,
  options: Array<{ label: string; value: string }>,
): JsonSchema {
  return {
    title,
    description: description ?? '',
    type: 'string',
    'x-decorator': 'FormItem',
    'x-component': 'Select',
    'x-component-props': { size: 'small' },
    enum: options,
  }
}

export function createBaseTypeSchema(
  baseType: string,
  title = '',
  description = '',
  needDecorator = false,
): JsonSchema {
  const schema: JsonSchema = {
    title,
    'x-decorator-props': withTooltip(description),
  }
  const componentProps: Record<string, unknown> = { size: 'small' }

  switch (baseType) {
    case 'Int':
      schema.type = 'number'
      schema['x-component'] = 'NumberPicker'
      componentProps.precision = 0
      schema['x-component-props'] = componentProps
      schema['x-decorator-props'] = {
        ...schema['x-decorator-props'],
        style: { width: 200 },
      }
      break
    case 'Decimal':
      schema.type = 'number'
      schema['x-component'] = 'NumberPicker'
      schema['x-component-props'] = componentProps
      schema['x-decorator-props'] = {
        ...schema['x-decorator-props'],
        style: { width: 200 },
      }
      break
    case 'Boolean':
      schema.type = 'boolean'
      schema['x-component'] = 'Switch'
      schema['x-component-props'] = componentProps
      break
    case 'Date':
      schema.type = 'string'
      schema['x-component'] = 'DatePicker'
      schema['x-component-props'] = {
        ...componentProps,
        format: 'YYYY-MM-DD',
      }
      break
    case 'Time':
      schema.type = 'string'
      schema['x-component'] = 'TimePicker'
      schema['x-component-props'] = {
        ...componentProps,
        format: 'HH:mm:ss',
      }
      break
    case 'Datetime':
      schema.type = 'string'
      schema['x-component'] = 'DatePicker'
      schema['x-component-props'] = {
        ...componentProps,
        showTime: true,
        format: 'YYYY-MM-DD HH:mm:ss',
      }
      break
    case 'DateRange':
      schema.type = 'array'
      schema['x-component'] = 'DatePicker.RangePicker'
      schema['x-component-props'] = {
        ...componentProps,
        format: 'YYYY-MM-DD',
      }
      break
    case 'TimeRange':
      schema.type = 'array'
      schema['x-component'] = 'TimePicker.RangePicker'
      schema['x-component-props'] = {
        ...componentProps,
        format: 'HH:mm:ss',
      }
      break
    case 'DatetimeRange':
      schema.type = 'array'
      schema['x-component'] = 'DatePicker.RangePicker'
      schema['x-component-props'] = {
        ...componentProps,
        showTime: true,
        format: 'YYYY-MM-DD HH:mm:ss',
      }
      break
    case 'Image':
    case 'Video':
    case 'File':
      schema.type = 'string'
      schema['x-component'] = 'FileUpload'
      schema['x-component-props'] = { uploadType: baseType, isArray: false }
      break
    case 'String':
      schema.type = 'string'
      schema['x-component'] = 'Input'
      schema['x-component-props'] = componentProps
      break
    default:
      throw new Error(`不支持的基本类型: ${baseType}`)
  }

  if (needDecorator) {
    schema['x-decorator'] = 'FormItem'
  }
  return schema
}

function createObjectProperties(
  children: SchemaNode[],
  resources: SchemaResources,
  modelPath: string[],
): Record<string, JsonSchema> {
  const properties: Record<string, JsonSchema> = {}
  children.forEach((child) => {
    Object.assign(properties, createNodeSchema(child, resources, modelPath))
  })
  return properties
}

function createObjectArraySchema(
  value: SchemaValueConfig,
  properties: Record<string, JsonSchema>,
): JsonSchema {
  return {
    type: 'array',
    title: value.name ?? value.key,
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
        'x-component': 'ArrayCards.Addition',
      },
    },
  }
}

function createBaseArraySchema(
  value: SchemaValueConfig,
  baseType: string,
): JsonSchema {
  if (FILE_TYPES.has(baseType)) {
    return {
      type: 'array',
      title: value.name ?? value.key,
      'x-decorator': 'FormItem',
      'x-decorator-props': withTooltip(value.description),
      'x-component': 'FileUpload',
      'x-component-props': { uploadType: baseType, isArray: true },
    }
  }

  return {
    type: 'array',
    title: value.name ?? value.key,
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
        'x-component-props': { style: { marginTop: 10 } },
      },
    },
  }
}

function createNodeSchema(
  node: SchemaNode,
  resources: SchemaResources,
  modelPath: string[],
): Record<string, JsonSchema> {
  const value = node.value
  const { baseType, enumKey, structureKey } = value.type

  if (structureKey) {
    if (modelPath.includes(structureKey)) {
      throw new Error(
        `检测到递归模型: ${[...modelPath, structureKey].join(' -> ')}`,
      )
    }
    const children = getStructureChildren(node, structureKey, resources)
    const properties = createObjectProperties(children, resources, [
      ...modelPath,
      structureKey,
    ])
    return {
      [value.key]: value.isArray
        ? createObjectArraySchema(value, properties)
        : {
            type: 'object',
            title: value.name ?? value.key,
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
    const options = (enumConfig.values ?? []).map((item) => ({
      label: item.label,
      value: item.value,
    }))
    if (value.isArray) {
      return {
        [value.key]: {
          type: 'array',
          title: value.name ?? value.key,
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
      [value.key]: createEnumSchema(
        value.name ?? value.key,
        value.description,
        options,
      ),
    }
  }

  if (!baseType) {
    throw new TypeError(`Schema 节点 ${value.key} 没有可用类型`)
  }
  return {
    [value.key]: value.isArray
      ? createBaseArraySchema(value, baseType)
      : createBaseTypeSchema(
          baseType,
          value.name ?? value.key,
          value.description,
          true,
        ),
  }
}

export function createSchema(
  node: SchemaNode,
  resources?: SchemaResources,
): Record<string, JsonSchema> {
  return createNodeSchema(node, validateResources(resources), [])
}

export function createWrappedSchema(
  node: SchemaNode,
  resources?: SchemaResources,
): JsonSchema {
  return wrapSchema(createSchema(node, resources), node.value)
}

export function isRangeType(baseType: string): boolean {
  return RANGE_TYPES.has(baseType)
}
