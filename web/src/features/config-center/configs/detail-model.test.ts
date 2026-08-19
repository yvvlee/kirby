import { describe, expect, it } from 'vitest'

import { formatConfigJSON } from './detail-model'

describe('config detail JSON preview', () => {
  it('formats valid serialized values and preserves empty values', () => {
    expect(formatConfigJSON('{"enabled":true}')).toBe('{\n  "enabled": true\n}')
    expect(formatConfigJSON('')).toBe('')
  })

  it('rejects malformed JSON instead of hiding it', () => {
    expect(() => formatConfigJSON('{')).toThrow(SyntaxError)
  })
})
