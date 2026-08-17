import axios from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  clearAccessToken,
  getAccessToken,
  getAccessTokenSnapshot,
  onSessionExpired,
  startAccessTokenSession,
} from '@/auth/token'
import { refreshAccessTokenSession } from '@/store/refresh-coordinator'

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

function deferred() {
  let resolve
  const promise = new Promise((done) => {
    resolve = done
  })
  return { promise, resolve }
}

afterEach(() => {
  clearAccessToken()
})

describe('API client', () => {
  it('冷启动刷新在途时，无令牌请求的 401 加入同一刷新', async () => {
    const requestStarted = deferred()
    const releaseRefresh = deferred()
    let bootstrapRefreshCalls = 0
    let adapterRefreshCalls = 0
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        adapterRefreshCalls += 1
        return success(config, { access_token: 'unexpected-token' })
      }
      if (config.headers.get('Authorization') === 'Bearer bootstrap-token') {
        return success(config, { ok: true })
      }
      requestStarted.resolve()
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    const bootstrapRefresh = refreshAccessTokenSession(async () => {
      bootstrapRefreshCalls += 1
      await releaseRefresh.promise
      return { access_token: 'bootstrap-token' }
    })
    const protectedRequest = client.get('/protected')
    await requestStarted.promise
    await new Promise((resolve) => setTimeout(resolve, 0))

    releaseRefresh.resolve()
    await bootstrapRefresh
    await expect(protectedRequest).resolves.toMatchObject({
      data: { ok: true },
    })

    expect(bootstrapRefreshCalls).toBe(1)
    expect(adapterRefreshCalls).toBe(0)
  })

  it('并发 401 只刷新一次并重放全部请求', async () => {
    let refreshCalls = 0
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        refreshCalls += 1
        expect(config.data).toBeNull()
        await new Promise((resolve) => setTimeout(resolve, 0))
        return success(config, { access_token: 'renewed-token' })
      }
      if (config.headers.get('Authorization') === 'Bearer renewed-token') {
        return success(config, { path: config.url })
      }
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    startAccessTokenSession('expired-token')

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
    startAccessTokenSession('expired-token')

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
    startAccessTokenSession('valid-token')

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
    startAccessTokenSession('expired-token')

    await expect(client.get('/protected')).rejects.toMatchObject({
      response: { status: 401 },
    })
    expect(getAccessToken()).toBeNull()
    expect(expired).toHaveBeenCalledTimes(1)
    unsubscribe()
  })

  it('错峰返回的旧 401 只使用新令牌重放，不再次刷新', async () => {
    const lateStarted = deferred()
    const releaseLate = deferred()
    let refreshCalls = 0
    let lateWaiting = false
    const adapter = async (config) => {
      const authorization = config.headers.get('Authorization')
      if (config.url === '/auth/refresh') {
        refreshCalls += 1
        return success(config, { access_token: 'renewed-token' })
      }
      if (
        config.url === '/late' &&
        authorization === 'Bearer expired-token' &&
        !lateWaiting
      ) {
        lateWaiting = true
        lateStarted.resolve()
        await releaseLate.promise
        return rejectWithStatus(401, config)
      }
      if (authorization === 'Bearer renewed-token') {
        return success(config, { path: config.url })
      }
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    startAccessTokenSession('expired-token')

    const lateRequest = client.get('/late')
    await lateStarted.promise
    await expect(client.get('/fast')).resolves.toMatchObject({
      data: { path: '/fast' },
    })
    releaseLate.resolve()
    await expect(lateRequest).resolves.toMatchObject({
      data: { path: '/late' },
    })

    expect(refreshCalls).toBe(1)
  })

  it('刷新失败后迟到的旧 401 不会再次刷新或重复过期', async () => {
    const lateStarted = deferred()
    const releaseLate = deferred()
    const expired = vi.fn()
    const unsubscribe = onSessionExpired(expired)
    let refreshCalls = 0
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        refreshCalls += 1
        return rejectWithStatus(401, config)
      }
      if (config.url === '/late') {
        lateStarted.resolve()
        await releaseLate.promise
      }
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    startAccessTokenSession('expired-token')

    const lateRejection = expect(client.get('/late')).rejects.toMatchObject({
      response: { status: 401 },
    })
    await lateStarted.promise
    await expect(client.get('/fast')).rejects.toMatchObject({
      response: { status: 401 },
    })
    releaseLate.resolve()
    await lateRejection

    expect(refreshCalls).toBe(1)
    expect(expired).toHaveBeenCalledTimes(1)
    expect(getAccessToken()).toBeNull()
    unsubscribe()
  })

  it('旧会话的迟到 401 绝不携带新用户令牌重放', async () => {
    const requestStarted = deferred()
    const releaseRequest = deferred()
    let refreshCalls = 0
    let replayedWithNewUser = false
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        refreshCalls += 1
        return success(config, { access_token: 'unexpected-token' })
      }
      if (config.headers.get('Authorization') === 'Bearer user-b-token') {
        replayedWithNewUser = true
        return success(config, {})
      }
      requestStarted.resolve()
      await releaseRequest.promise
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    startAccessTokenSession('user-a-token')
    const oldSessionGeneration = getAccessTokenSnapshot().sessionGeneration

    const oldRequest = client.get('/late')
    await requestStarted.promise
    clearAccessToken()
    startAccessTokenSession('user-b-token')
    releaseRequest.resolve()

    await expect(oldRequest).rejects.toMatchObject({
      response: { status: 401 },
    })
    expect(getAccessTokenSnapshot()).toMatchObject({
      token: 'user-b-token',
      sessionGeneration: oldSessionGeneration + 2,
    })
    expect(replayedWithNewUser).toBe(false)
    expect(refreshCalls).toBe(0)
  })

  it('应用级刷新操作与 401 处理共享同一个 Promise', async () => {
    const refreshStarted = deferred()
    const releaseRefresh = deferred()
    let coordinatorCalls = 0
    let adapterRefreshCalls = 0
    const adapter = async (config) => {
      if (config.url === '/auth/refresh') {
        adapterRefreshCalls += 1
        return success(config, { access_token: 'adapter-token' })
      }
      if (config.headers.get('Authorization') === 'Bearer shared-token') {
        return success(config, { ok: true })
      }
      return rejectWithStatus(401, config)
    }
    const client = createApiClient({ adapter })
    startAccessTokenSession('expired-token')
    const sharedRefresh = refreshAccessTokenSession(async () => {
      coordinatorCalls += 1
      refreshStarted.resolve()
      await releaseRefresh.promise
      return { access_token: 'shared-token' }
    })
    await refreshStarted.promise

    const protectedRequest = client.get('/protected')
    releaseRefresh.resolve()
    await sharedRefresh
    await expect(protectedRequest).resolves.toMatchObject({
      data: { ok: true },
    })

    expect(coordinatorCalls).toBe(1)
    expect(adapterRefreshCalls).toBe(0)
  })
})
