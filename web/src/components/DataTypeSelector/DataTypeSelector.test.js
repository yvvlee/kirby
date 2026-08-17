import { describe, expect, it } from 'vitest'

import { buildDataTypeGroups } from './DataTypeSelector.vue'

describe('buildDataTypeGroups', () => {
  it('只组合基本类型、当前枚举和当前模型', () => {
    const groups = buildDataTypeGroups({
      models: [{ key: 'Server', name: '服务器' }],
      enums: [{ key: 'State', name: '状态' }],
      limitedModels: [],
      limit: false,
    })

    expect(groups.map((group) => group.label)).toEqual([
      '基本类型',
      '枚举',
      '模型',
    ])
    expect(groups[1].options).toEqual([
      { label: '状态', value: '{"enumKey":"State"}' },
    ])
    expect(groups[2].options).toEqual([
      { label: '服务器', value: '{"structureKey":"Server"}' },
    ])
  })

  it('限制模式下只显示可引用模型', () => {
    const groups = buildDataTypeGroups({
      models: [{ key: 'Blocked', name: '不可选' }],
      enums: [],
      limitedModels: [{ key: 'Allowed', name: '可选' }],
      limit: true,
    })

    expect(groups.at(-1).options).toEqual([
      { label: '可选', value: '{"structureKey":"Allowed"}' },
    ])
  })
})
