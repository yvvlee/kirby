import { describe, expect, it } from 'vitest'

import { unwrapValue, wrapSchema, wrapValue } from './wrapper.js'

describe('schema wrapper', () => {
  it('包装根级基本类型', () => {
    const config = {
      key: 'port',
      type: { baseType: 'Int' },
      isArray: false,
    }
    expect(wrapSchema({ port: { type: 'number' } }, config)).toEqual({
      type: 'object',
      properties: { port: { type: 'number' } },
    })
    expect(unwrapValue(wrapValue(8080, config), config)).toBe(8080)
  })

  it('不包装根级对象', () => {
    const config = {
      key: 'server',
      type: { structureKey: 'Server' },
      isArray: false,
    }
    const value = { host: '127.0.0.1' }
    expect(wrapSchema({ server: { type: 'object' } }, config)).toEqual({
      type: 'object',
    })
    expect(unwrapValue(wrapValue(value, config), config)).toBe(value)
  })
})
