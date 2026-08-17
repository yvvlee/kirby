import { describe, expect, it } from 'vitest'

import { formatDiffValue } from './format.js'

describe('formatDiffValue', () => {
  it('使用稳定的两空格 JSON 展示差异', () => {
    expect(formatDiffValue({ enabled: true })).toBe('{\n  "enabled": true\n}')
  })

  it('将未定义值显示为 null', () => {
    expect(formatDiffValue(undefined)).toBe('null')
  })

  it('拒绝无法序列化的值', () => {
    expect(() => formatDiffValue(() => {})).toThrow(
      '差异对比值无法转换为 JSON',
    )
  })
})
