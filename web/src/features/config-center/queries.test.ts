import { describe, expect, it } from 'vitest'

import { normalizeConfigDetail, normalizeConfigList, normalizeEnums, normalizeProjectDetail } from './queries'

describe('configuration response normalization', () => {
  it('extracts project and config wrappers', () => {
    expect(normalizeProjectDetail({ project: { id: 7, key: 'Demo', name: 'Demo', version: 1 } }).id).toBe(7)
    expect(normalizeProjectDetail({ project: { id: '9007199254740993', key: 'Demo', name: 'Demo', version: 1 } }).id).toBe('9007199254740993')
    expect(normalizeConfigList({ list: [{ config: { id: 31, key: 'Flags', version: 1 }, isReleased: true }] })).toEqual([
      expect.objectContaining({ id: 31, key: 'Flags', isReleased: true }),
    ])
  })

  it('normalizes protobuf tree fields and array flags', () => {
    const detail = normalizeConfigDetail({
      config: { id: 31, key: 'Flags', version: 2, value: 'true', type: { base_type: 'BOOLEAN' }, is_array: true },
      tree: { value: { key: 'Flags', type: { base_type: 'BOOLEAN' } }, children: [] },
    })
    expect(detail.config.isArray).toBe(true)
    expect(detail.tree?.value.type).toEqual({ baseType: 'Boolean' })
  })

  it('fails fast on malformed wrappers', () => {
    expect(() => normalizeProjectDetail({})).toThrow('项目详情响应缺少 project')
    expect(() => normalizeProjectDetail({ project: { id: '0', key: 'Demo', name: 'Demo' } })).toThrow('项目详情响应中的 project 不完整')
    expect(() => normalizeConfigList({ list: [{}] })).toThrow('配置列表项缺少 config')
    expect(() => normalizeEnums({ list: [{ key: 'State' }] })).toThrow('枚举响应缺少 values')
  })
})
