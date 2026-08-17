import client from './client'
import { getAccessToken } from '@/auth/token'
import { refreshAccessTokenSession } from '@/store/refresh-coordinator'

export async function login(credentials) {
  const { data } = await client.post('/auth/login', credentials, {
    skipAuthRefresh: true,
    skipAccessToken: true,
  })
  return data
}

export async function refreshSession() {
  const { reply } = await refreshAccessTokenSession(async () => {
    const { data } = await client.post('/auth/refresh', null, {
      skipAuthRefresh: true,
      skipAccessToken: true,
    })
    return data
  })
  return reply
}

export async function logout(accessToken = getAccessToken()) {
  await client.post('/auth/logout', null, {
    headers: accessToken
      ? { Authorization: `Bearer ${accessToken}` }
      : undefined,
    skipAuthRefresh: true,
    skipAccessToken: true,
  })
}

export async function getCurrentUser() {
  const { data } = await client.get('/auth/me')
  return data
}
