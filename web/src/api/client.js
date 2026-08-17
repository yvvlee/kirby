import axios from 'axios'

import { getAccessTokenSnapshot } from '@/auth/token'
import {
  expireAccessTokenSession,
  refreshAccessTokenSession,
} from '@/store/refresh-coordinator'

const API_BASE_URL = '/api'

function isRefreshableUnauthorized(error) {
  return error.response?.status === 401 && !error.config?.skipAuthRefresh
}

function setAuthorization(config, token) {
  if (typeof config.headers?.set === 'function') {
    config.headers.set('Authorization', `Bearer ${token}`)
    return
  }
  config.headers = {
    ...config.headers,
    Authorization: `Bearer ${token}`,
  }
}

export function createApiClient(options = {}) {
  const commonOptions = {
    baseURL: options.baseURL || API_BASE_URL,
    withCredentials: true,
  }
  if (options.adapter) {
    commonOptions.adapter = options.adapter
  }

  const client = axios.create(commonOptions)
  const refreshClient = axios.create(commonOptions)

  client.interceptors.request.use((config) => {
    const snapshot = getAccessTokenSnapshot()
    config._kirbyAccessTokenGeneration = snapshot.accessTokenGeneration
    config._kirbySessionGeneration = snapshot.sessionGeneration
    if (snapshot.token && !config.skipAccessToken) {
      setAuthorization(config, snapshot.token)
    }
    return config
  })

  function refreshAccessToken() {
    return refreshAccessTokenSession(async () => {
      const { data } = await refreshClient.post('/auth/refresh', null)
      return data
    })
  }

  client.interceptors.response.use(
    (value) => value,
    async (error) => {
      if (!isRefreshableUnauthorized(error)) {
        throw error
      }

      const originalConfig = error.config
      const requestSessionGeneration =
        originalConfig._kirbySessionGeneration
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
