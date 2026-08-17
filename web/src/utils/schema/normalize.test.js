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

  it('无效 JSON 使用结构默认值', () => {
    expect(normalizeData(profileNode, '{', resources)).toEqual({
      name: '',
      age: 0,
      active: false,
      period: [],
    })
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
})
