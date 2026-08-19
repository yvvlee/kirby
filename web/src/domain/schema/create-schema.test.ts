import { describe, expect, it } from 'vitest'

import {
  createBaseTypeSchema,
  createSchema,
  createWrappedSchema,
} from './create-schema'
import type { SchemaNode } from './types'

function node(
  key: string,
  type: SchemaNode['value']['type'],
  options: Partial<Omit<SchemaNode['value'], 'key' | 'type'>> & {
    children?: SchemaNode[]
  } = {},
): SchemaNode {
  return {
    value: {
      key,
      name: options.name ?? key,
      description: options.description ?? '',
      isArray: Boolean(options.isArray),
      type,
    },
    children: options.children ?? [],
  }
}

describe('createSchema', () => {
  it('expands nested structures from explicit model resources', () => {
    const schema = createWrappedSchema(
      node('address', { structureKey: 'Address' }),
      {
        models: [
          {
            key: 'Address',
            fields: [
              { key: 'city', name: '城市', type: { baseType: 'String' } },
            ],
          },
        ],
        enums: [],
      },
    )

    expect(schema.type).toBe('object')
    expect(schema.properties?.city).toMatchObject({
      title: '城市',
      type: 'string',
      'x-component': 'Input',
    })
  })

  it('fails fast for recursive models', () => {
    expect(() =>
      createSchema(node('tree', { structureKey: 'TreeNode' }), {
        models: [
          {
            key: 'TreeNode',
            fields: [
              {
                key: 'children',
                isArray: true,
                type: { structureKey: 'TreeNode' },
              },
            ],
          },
        ],
        enums: [],
      }),
    ).toThrow('检测到递归模型: TreeNode -> TreeNode')
  })

  it('creates enum options and rejects a missing enum', () => {
    const schema = createSchema(node('color', { enumKey: 'Color' }), {
      models: [],
      enums: [
        {
          key: 'Color',
          values: [
            { label: '红色', value: 'red' },
            { label: '蓝色', value: 'blue' },
          ],
        },
      ],
    })

    expect(schema.color?.enum).toEqual([
      { label: '红色', value: 'red' },
      { label: '蓝色', value: 'blue' },
    ])
    expect(() =>
      createSchema(node('missing', { enumKey: 'Missing' }), {
        models: [],
        enums: [],
      }),
    ).toThrow('找不到枚举: Missing')
  })

  it('requires explicit resources', () => {
    expect(() => createSchema(node('name', { baseType: 'String' }))).toThrow(
      'createSchema 需要显式传入 { models, enums }',
    )
  })
})

describe('createBaseTypeSchema', () => {
  it('uses React Formily component names and Day.js formats', () => {
    expect(createBaseTypeSchema('Int')).toMatchObject({
      type: 'number',
      'x-component': 'NumberPicker',
    })
    expect(createBaseTypeSchema('DateRange')).toMatchObject({
      type: 'array',
      'x-component': 'DatePicker.RangePicker',
      'x-component-props': { format: 'YYYY-MM-DD' },
    })
    expect(createBaseTypeSchema('Datetime')).toMatchObject({
      'x-component-props': {
        showTime: true,
        format: 'YYYY-MM-DD HH:mm:ss',
      },
    })
  })

  it('rejects an unknown base type', () => {
    expect(() => createBaseTypeSchema('UnknownType')).toThrow(
      '不支持的基本类型: UnknownType',
    )
  })
})
