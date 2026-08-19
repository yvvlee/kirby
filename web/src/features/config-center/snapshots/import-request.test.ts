import { describe, expect, it, vi } from 'vitest'

import { nextImportRequestIdentity } from './import-request'

describe('snapshot import idempotency', () => {
  it('reuses a key for retries of the same request', () => {
    const createKey = vi.fn(() => 'new-key')
    expect(nextImportRequestIdentity({ signature: 'same', key: 'stable-key' }, 'same', createKey)).toEqual({ signature: 'same', key: 'stable-key' })
    expect(createKey).not.toHaveBeenCalled()
  })

  it('creates a new key only when business fields change', () => {
    expect(nextImportRequestIdentity({ signature: 'old', key: 'stable-key' }, 'changed', () => 'new-key')).toEqual({ signature: 'changed', key: 'new-key' })
  })

  it('requires a non-empty request signature', () => {
    expect(() => nextImportRequestIdentity({ signature: '', key: '' }, '', () => 'key')).toThrow('快照导入请求签名不能为空')
  })
})
