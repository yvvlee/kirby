import axios from 'axios'

import {
  clearAccessToken,
  getAccessToken,
  notifySessionExpired,
  setAccessToken,
} from '@/auth/token'

const API_BASE_URL = '/api'

function readAccessToken(data) {
  const token = data?.access_token
  if (typeof token !== 'string' || token.length === 0) {
    throw new TypeError('refresh response does not contain access_token')
  }
  return token
}

function shouldRefresh(error) {
  return (
    error.response?.status === 401 &&
    !error.config?.skipAuthRefresh &&
    !error.config?._kirbyRetried
  )
}

function isRejectedRetry(error) {
  return error.response?.status === 401 && error.config?._kirbyRetried
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
  let refreshPromise = null
  let expirationPromise = null

  client.interceptors.request.use((config) => {
    const token = getAccessToken()
    if (token && !config.skipAccessToken) {
      setAuthorization(config, token)
    }
    return config
  })

  function expireSession(error) {
    if (!expirationPromise) {
      clearAccessToken()
      expirationPromise = notifySessionExpired(error).finally(() => {
        expirationPromise = null
      })
    }
    return expirationPromise
  }

  function refreshAccessToken() {
    if (!refreshPromise) {
      refreshPromise = refreshClient
        .post('/auth/refresh')
        .then(({ data }) => {
          const token = readAccessToken(data)
          setAccessToken(token)
          return token
        })
        .catch(async (error) => {
          await expireSession(error)
          throw error
        })
        .finally(() => {
          refreshPromise = null
        })
    }
    return refreshPromise
  }

  client.interceptors.response.use(
    (value) => value,
    async (error) => {
      if (isRejectedRetry(error)) {
        await expireSession(error)
        throw error
      }
      if (!shouldRefresh(error)) {
        throw error
      }

      const originalConfig = error.config
      originalConfig._kirbyRetried = true
      const token = await refreshAccessToken()
      setAuthorization(originalConfig, token)
      return client.request(originalConfig)
    },
  )

  return client
}

const apiClient = createApiClient()

export default apiClient
