import { describe, expect, it } from 'vitest'

import { normalizeData } from './normalize'
import type { SchemaNode, SchemaResources } from './types'

const resources: SchemaResources = {
  models: [
    {
      key: 'Profile',
      fields: [
        { key: 'name', type: { baseType: 'String' } },
        { key: 'age', type: { baseType: 'Int' } },
        { key: 'active', type: { baseType: 'Boolean' } },
        { key: 'period', type: { baseType: 'DateRange' } },
      ],
    },
  ],
  enums: [{ key: 'State', values: [{ label: '启用', value: 'enabled' }] }],
}

const profileNode: SchemaNode = {
  value: {
    key: 'profile',
    type: { structureKey: 'Profile' },
    isArray: false,
  },
  children: [],
}

describe('normalizeData', () => {
  it('removes unknown fields and repairs invalid values', () => {
    expect(
      normalizeData(
        profileNode,
        JSON.stringify({
          name: null,
          age: 1.5,
          active: 'yes',
          period: ['2026-08-01'],
          ignored: true,
        }),
        resources,
      ),
    ).toEqual({ name: '', age: 0, active: false, period: [] })
  })

  it('uses structural defaults for an empty value', () => {
    expect(normalizeData(profileNode, '', resources)).toEqual({
      name: '',
      age: 0,
      active: false,
      period: [],
    })
  })

  it('rejects invalid JSON and unknown base types', () => {
    expect(() => normalizeData(profileNode, '{', resources)).toThrow(
      '配置项 profile 的值不是合法 JSON',
    )
    const unknownNode: SchemaNode = {
      value: {
        key: 'unknown',
        type: { baseType: 'UnknownType' },
      },
    }
    expect(() => normalizeData(unknownNode, '"value"', resources)).toThrow(
      '不支持的基本类型: UnknownType',
    )
  })

  it('rejects non-string serialized input', () => {
    expect(() => normalizeData(profileNode, {}, resources)).toThrow(
      'normalizeData 的 data 必须是 JSON 字符串',
    )
  })
})
