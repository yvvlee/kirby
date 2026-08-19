import axios, {
  type AxiosAdapter,
  type AxiosError,
  type AxiosInstance,
  type AxiosRequestConfig,
  type CreateAxiosDefaults,
  type InternalAxiosRequestConfig,
} from 'axios'

import { getAccessTokenSnapshot } from '@/auth/token'
import {
  expireAccessTokenSession,
  refreshAccessTokenSession,
} from '@/auth/refresh-coordinator'

const API_BASE_URL = '/api'

export type KirbyRequestState = {
  skipAuthRefresh?: boolean
  skipAccessToken?: boolean
  _kirbyAccessTokenGeneration?: number
  _kirbySessionGeneration?: number
  _kirbyRetried?: boolean
}

export type KirbyRequestOptions = AxiosRequestConfig & KirbyRequestState

type KirbyRequestConfig = InternalAxiosRequestConfig & KirbyRequestState

type RefreshableUnauthorized = AxiosError & {
  config: KirbyRequestConfig
}

export type ApiClientOptions = {
  baseURL?: string
  adapter?: AxiosAdapter
}

function asKirbyConfig(
  config: InternalAxiosRequestConfig,
): KirbyRequestConfig {
  return config as KirbyRequestConfig
}

function isRefreshableUnauthorized(
  error: unknown,
): error is RefreshableUnauthorized {
  if (!axios.isAxiosError(error) || !error.config) {
    return false
  }
  const config = asKirbyConfig(error.config)
  return error.response?.status === 401 && !config.skipAuthRefresh
}

function setAuthorization(config: KirbyRequestConfig, token: string): void {
  config.headers.set('Authorization', `Bearer ${token}`)
}

export function createApiClient(options: ApiClientOptions = {}): AxiosInstance {
  const commonOptions: CreateAxiosDefaults = {
    baseURL: options.baseURL ?? API_BASE_URL,
    withCredentials: true,
  }
  if (options.adapter) {
    commonOptions.adapter = options.adapter
  }

  const client = axios.create(commonOptions)
  const refreshClient = axios.create(commonOptions)

  client.interceptors.request.use((config) => {
    const kirbyConfig = asKirbyConfig(config)
    const snapshot = getAccessTokenSnapshot()
    kirbyConfig._kirbyAccessTokenGeneration = snapshot.accessTokenGeneration
    kirbyConfig._kirbySessionGeneration = snapshot.sessionGeneration
    if (snapshot.token && !kirbyConfig.skipAccessToken) {
      setAuthorization(kirbyConfig, snapshot.token)
    }
    return kirbyConfig
  })

  function refreshAccessToken() {
    return refreshAccessTokenSession(async () => {
      const { data } = await refreshClient.post<unknown>('/auth/refresh', null)
      return data
    })
  }

  client.interceptors.response.use(
    (value) => value,
    async (error: unknown) => {
      if (!isRefreshableUnauthorized(error)) {
        throw error
      }

      const originalConfig = error.config
      const requestSessionGeneration = originalConfig._kirbySessionGeneration
      const requestAccessTokenGeneration =
        originalConfig._kirbyAccessTokenGeneration
      let current = getAccessTokenSnapshot()

      if (requestSessionGeneration !== current.sessionGeneration) {
        throw error
      }
      if (current.token === null) {
        if (!current.refreshAllowed) {
          await expireAccessTokenSession(error, current.sessionGeneration)
          throw error
        }
      } else if (
        requestAccessTokenGeneration !== current.accessTokenGeneration
      ) {
        originalConfig._kirbyRetried = true
        setAuthorization(originalConfig, current.token)
        return client.request(originalConfig)
      }
      if (originalConfig._kirbyRetried) {
        await expireAccessTokenSession(error, current.sessionGeneration)
        throw error
      }

      const refresh = await refreshAccessToken()
      current = getAccessTokenSnapshot()
      if (
        refresh.started.sessionGeneration !== requestSessionGeneration ||
        refresh.completed.sessionGeneration !== current.sessionGeneration ||
        current.token === null
      ) {
        throw error
      }
      originalConfig._kirbyRetried = true
      setAuthorization(originalConfig, current.token)
      return client.request(originalConfig)
    },
  )

  return client
}

const apiClient = createApiClient()

export default apiClient
