import { describe, expect, it } from 'vitest'

import { requireKeyList, requireSecretReply } from './model'

describe('API Key secret boundary', () => {
  it('accepts metadata lists without full secrets', () => {
    expect(requireKeyList({ list: [{ id: 1, name: 'production', secretSuffix: '1234' }] })).toHaveLength(1)
  })

  it('rejects a full secret in list data', () => {
    expect(() => requireKeyList({ list: [{ id: 1, secret: 'must-not-cache' }] })).toThrow('API Key 列表不得包含完整 Secret')
  })

  it('requires both metadata and a one-time secret', () => {
    expect(requireSecretReply({ apiKey: { id: 1 }, secret: 'one-time-secret' })).toBe('one-time-secret')
    expect(() => requireSecretReply({ apiKey: { id: 1 } })).toThrow('API Key 响应缺少一次性 Secret')
  })
})
