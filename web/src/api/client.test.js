import axios from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  clearAccessToken,
  getAccessToken,
  onSessionExpired,
  setAccessToken,
} from '@/auth/token'

import { createApiClient } from './client'

function rejectWithStatus(status, config, data = {}) {
  const response = {
    status,
    data,
    config,
    headers: {},
    statusText: String(status),
  }
  throw new axios.AxiosError(
    `request failed with status ${status}`,
    axios.AxiosError.ERR_BAD_RESPONSE,
    config,
    null,
    response,
  )
}

function success(config, data) {
  return {
    status: 200,
    data,
    config,
    headers: {},
    statusText: 'OK',
  }
}

afterEach(() => {
  clearAccessToken()
})

describe('API client', () => {
  it('并发 401 只刷新一次并重放全部请求', async () => {
    let refreshCalls = 0
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        refreshCalls += 1
        await new Promise((resolve) => setTimeout(resolve, 0))
        return success(config, { access_token: 'renewed-token' })
      }
      if (config.headers.get('Authorization') === 'Bearer renewed-token') {
        return success(config, { path: config.url })
      }
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    setAccessToken('expired-token')

    const [first, second] = await Promise.all([
      client.get('/first'),
      client.get('/second'),
    ])

    expect(refreshCalls).toBe(1)
    expect(first.data).toEqual({ path: '/first' })
    expect(second.data).toEqual({ path: '/second' })
    expect(getAccessToken()).toBe('renewed-token')
  })

  it('刷新失败后清空内存令牌并只发出一次过期事件', async () => {
    const expired = vi.fn()
    const unsubscribe = onSessionExpired(expired)
    const adapter = (config) => rejectWithStatus(401, config)
    const client = createApiClient({ adapter })
    setAccessToken('expired-token')

    await expect(client.get('/protected')).rejects.toMatchObject({
      response: { status: 401 },
    })

    expect(getAccessToken()).toBeNull()
    expect(expired).toHaveBeenCalledTimes(1)
    unsubscribe()
  })

  it('403 直接返回且不会尝试刷新', async () => {
    let refreshCalls = 0
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        refreshCalls += 1
      }
      return rejectWithStatus(403, config)
    }
    const client = createApiClient({ adapter })
    setAccessToken('valid-token')

    await expect(client.get('/forbidden')).rejects.toMatchObject({
      response: { status: 403 },
    })
    expect(refreshCalls).toBe(0)
    expect(getAccessToken()).toBe('valid-token')
  })

  it('刷新后的令牌仍被拒绝时结束会话', async () => {
    const expired = vi.fn()
    const unsubscribe = onSessionExpired(expired)
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        return success(config, { access_token: 'rejected-token' })
      }
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    setAccessToken('expired-token')

    await expect(client.get('/protected')).rejects.toMatchObject({
      response: { status: 401 },
    })
    expect(getAccessToken()).toBeNull()
    expect(expired).toHaveBeenCalledTimes(1)
    unsubscribe()
  })
})
