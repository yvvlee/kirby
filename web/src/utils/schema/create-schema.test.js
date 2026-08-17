import { describe, expect, it } from 'vitest'

import {
  createBaseTypeSchema,
  createSchema,
  createWrappedSchema,
} from './create-schema.js'

const node = (key, type, options = {}) => ({
  value: {
    key,
    name: options.name || key,
    description: options.description || '',
    isArray: Boolean(options.isArray),
    type,
  },
  children: options.children || [],
})

describe('createSchema', () => {
  it('通过显式的模型列表展开嵌套结构', () => {
    const resources = {
      models: [
        {
          key: 'Address',
          fields: [
            {
              key: 'city',
              name: '城市',
              type: { baseType: 'String' },
            },
          ],
        },
      ],
      enums: [],
    }
    const schema = createWrappedSchema(
      node('address', { structureKey: 'Address' }),
      resources,
    )

    expect(schema.type).toBe('object')
    expect(schema.properties.city).toMatchObject({
      title: '城市',
      type: 'string',
      'x-component': 'Input',
    })
  })

  it('对递归模型快速报错', () => {
    const resources = {
      models: [
        {
          key: 'TreeNode',
          fields: [
            {
              key: 'children',
              name: '子节点',
              isArray: true,
              type: { structureKey: 'TreeNode' },
            },
          ],
        },
      ],
      enums: [],
    }

    expect(() =>
      createSchema(node('tree', { structureKey: 'TreeNode' }), resources),
    ).toThrow('检测到递归模型: TreeNode -> TreeNode')
  })

  it('从显式的枚举列表生成选项', () => {
    const resources = {
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
    }
    const schema = createSchema(node('color', { enumKey: 'Color' }), resources)

    expect(schema.color.enum).toEqual([
      { label: '红色', value: 'red' },
      { label: '蓝色', value: 'blue' },
    ])
  })

  it('枚举不存在时不生成空选项', () => {
    expect(() =>
      createSchema(node('color', { enumKey: 'Missing' }), {
        models: [],
        enums: [],
      }),
    ).toThrow('找不到枚举: Missing')
  })

  it('要求显式传入模型和枚举', () => {
    expect(() => createSchema(node('name', { baseType: 'String' }))).toThrow(
      'createSchema 需要显式传入 { models, enums }',
    )
  })
})

describe('createBaseTypeSchema', () => {
  it('为日期范围使用数组类型和固定输出格式', () => {
    expect(createBaseTypeSchema('DateRange')).toMatchObject({
      type: 'array',
      'x-component': 'DatePicker',
      'x-component-props': {
        type: 'daterange',
        valueFormat: 'yyyy-MM-dd',
      },
    })
  })

  it('拒绝未知基本类型', () => {
    expect(() => createBaseTypeSchema('UnknownType')).toThrow(
      '不支持的基本类型: UnknownType',
    )
  })
})
