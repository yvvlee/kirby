import { describe, expect, it } from 'vitest'

import { normalizeData } from './normalize.js'

const resources = {
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

const profileNode = {
  value: {
    key: 'profile',
    type: { structureKey: 'Profile' },
    isArray: false,
  },
  children: [],
}

describe('normalizeData', () => {
  it('删除多余字段并修复空值和错误类型', () => {
    const normalized = normalizeData(
      profileNode,
      JSON.stringify({
        name: null,
        age: 1.5,
        active: 'yes',
        period: ['2026-08-01'],
        ignored: true,
      }),
      resources,
    )

    expect(normalized).toEqual({
      name: '',
      age: 0,
      active: false,
      period: [],
    })
  })

  it('空字符串使用结构默认值', () => {
    expect(normalizeData(profileNode, '', resources)).toEqual({
      name: '',
      age: 0,
      active: false,
      period: [],
    })
  })

  it('拒绝非空的非法 JSON 并标明配置项', () => {
    expect(() => normalizeData(profileNode, '{', resources)).toThrow(
      '配置项 profile 的值不是合法 JSON',
    )
  })

  it('保留合法的枚举字符串', () => {
    const enumNode = {
      value: {
        key: 'state',
        type: { enumKey: 'State' },
        isArray: false,
      },
    }
    expect(normalizeData(enumNode, '"enabled"', resources)).toBe('enabled')
    expect(normalizeData(enumNode, '1', resources)).toBe('')
  })

  it('拒绝非字符串 JSON 输入', () => {
    expect(() => normalizeData(profileNode, {}, resources)).toThrow(
      'normalizeData 的 data 必须是 JSON 字符串',
    )
  })

  it('拒绝未知基本类型', () => {
    const unknownNode = {
      value: {
        key: 'unknown',
        type: { baseType: 'UnknownType' },
        isArray: false,
      },
    }
    expect(() => normalizeData(unknownNode, '"value"', resources)).toThrow(
      '不支持的基本类型: UnknownType',
    )

    unknownNode.value.isArray = true
    expect(() => normalizeData(unknownNode, '', resources)).toThrow(
      '不支持的基本类型: UnknownType',
    )
  })
})
