import { describe, expect, it } from 'vitest'

import { requireEnumValues, requireModelFields } from './editor-validation'

describe('configuration editor validation', () => {
  it('accepts valid model fields and enum values', () => {
    expect(() => requireModelFields([{ key: 'createdAt', name: '创建时间', description: '', isArray: false, type: '{"baseType":"Datetime"}' }])).not.toThrow()
    expect(() => requireEnumValues([{ label: '启用', value: 'ENABLED' }])).not.toThrow()
  })

  it('rejects missing and duplicate model fields', () => {
    expect(() => requireModelFields([])).toThrow('模型至少需要一个字段')
    expect(() => requireModelFields([
      { key: 'name', name: '名称', description: '', isArray: false, type: '{"baseType":"String"}' },
      { key: 'name', name: '重复', description: '', isArray: false, type: '{"baseType":"String"}' },
    ])).toThrow('字段标识重复: name')
  })

  it('rejects malformed editor types', () => {
    expect(() => requireModelFields([{ key: 'name', name: '名称', description: '', isArray: false, type: '{' }])).toThrow('字段类型不是合法 JSON')
  })

  it('rejects invalid and duplicate enum values', () => {
    expect(() => requireEnumValues([])).toThrow('枚举至少需要一个值')
    expect(() => requireEnumValues([{ label: '启用', value: 'enabled' }])).toThrow('必须使用大写字母')
    expect(() => requireEnumValues([{ label: '一', value: 'SAME' }, { label: '二', value: 'SAME' }])).toThrow('枚举值重复: SAME')
  })
})
