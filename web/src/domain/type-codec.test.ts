import { describe, expect, it } from 'vitest'

import { normalizeModel, normalizeTree, parseEditorType, stringifyEditorType, toApiType, toEditorType } from './type-codec'

describe('configuration type codec', () => {
  it('converts protobuf enum names and references in both directions', () => {
    expect(toEditorType({ base_type: 'DATETIME_RANGE' })).toEqual({ baseType: 'DatetimeRange' })
    expect(toApiType({ baseType: 'DatetimeRange' })).toEqual({ base_type: 'DATETIME_RANGE' })
    expect(toApiType({ structureKey: 'User' })).toEqual({ structure_key: 'User' })
    expect(toApiType({ enumKey: 'Status' })).toEqual({ enum_key: 'Status' })
  })

  it('normalizes models and nested trees', () => {
    expect(normalizeModel({ key: 'User', fields: [{ key: 'name', is_array: true, type: { base_type: 'STRING' } }] }).fields?.[0]).toMatchObject({ isArray: true, type: { baseType: 'String' } })
    expect(normalizeTree({ value: { key: 'root', type: { structure_key: 'User' } }, children: [{ value: { key: 'active', type: { base_type: 'BOOLEAN' } } }] }).children?.[0]?.value.type).toEqual({ baseType: 'Boolean' })
  })

  it('rejects unknown and malformed types', () => {
    expect(() => toEditorType({ base_type: 'MONEY' })).toThrow('不支持的 API 基本类型: MONEY')
    expect(() => parseEditorType('{')).toThrow('字段类型不是合法 JSON')
    expect(() => stringifyEditorType({})).toThrow('字段类型没有 base_type、structure_key 或 enum_key')
  })
})
