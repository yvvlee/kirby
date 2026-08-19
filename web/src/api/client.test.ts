import axios, {
  AxiosError,
  AxiosHeaders,
  type AxiosAdapter,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import { afterEach, describe, expect, it } from 'vitest'

import {
  clearAccessToken,
  getAccessToken,
  startAccessTokenSession,
} from '@/auth/token'

import { createApiClient } from './client'
import { getApiErrorMessage } from './errors'

function success<T>(
  config: InternalAxiosRequestConfig,
  data: T,
): AxiosResponse<T> {
  return {
    status: 200,
    data,
    config,
    headers: new AxiosHeaders(),
    statusText: 'OK',
  }
}

function rejectWithStatus(
  status: number,
  config: InternalAxiosRequestConfig,
): never {
  const response: AxiosResponse = {
    status,
    data: {},
    config,
    headers: new AxiosHeaders(),
    statusText: String(status),
  }
  throw new AxiosError(
    `request failed with status ${status}`,
    AxiosError.ERR_BAD_RESPONSE,
    config,
    undefined,
    response,
  )
}

function deferred(): {
  promise: Promise<void>
  resolve: () => void
} {
  let resolvePromise: (() => void) | undefined
  const promise = new Promise<void>((resolve) => {
    resolvePromise = resolve
  })
  return {
    promise,
    resolve: () => resolvePromise?.(),
  }
}

afterEach(() => {
  clearAccessToken()
})

describe('API client', () => {
  it('refreshes once and replays concurrent unauthorized requests', async () => {
    let refreshCalls = 0
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/auth/refresh') {
        refreshCalls += 1
        await Promise.resolve()
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

  it('does not replay a late response from an old session', async () => {
    const requestStarted = deferred()
    const releaseRequest = deferred()
    let replayedWithNewUser = false
    const adapter: AxiosAdapter = async (config) => {
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

    const oldRequest = client.get('/late')
    await requestStarted.promise
    clearAccessToken()
    startAccessTokenSession('user-b-token')
    releaseRequest.resolve()

    await expect(oldRequest).rejects.toMatchObject({
      response: { status: 401 },
    })
    expect(replayedWithNewUser).toBe(false)
    expect(getAccessToken()).toBe('user-b-token')
  })

  it('does not refresh a forbidden response', async () => {
    let refreshCalls = 0
    const adapter: AxiosAdapter = async (config) => {
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
})

describe('getApiErrorMessage', () => {
  it('reads an API detail from an unknown error', () => {
    const error = new axios.AxiosError('bad request')
    error.response = {
      status: 400,
      data: { detail: '名称已存在' },
      config: { headers: new AxiosHeaders() },
      headers: new AxiosHeaders(),
      statusText: 'Bad Request',
    }

    expect(getApiErrorMessage(error)).toBe('名称已存在')
  })
})
