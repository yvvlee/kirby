import { refreshAccessTokenSession } from '@/auth/refresh-coordinator'
import { getAccessToken } from '@/auth/token'

import client, { type KirbyRequestOptions } from './client'
import type { ApiObject, User } from './types'

export type LoginCredentials = {
  username: string
  password: string
}

export type AuthenticationReply = ApiObject & {
  access_token: string
  user: User
}

const anonymousRequest: KirbyRequestOptions = {
  skipAuthRefresh: true,
  skipAccessToken: true,
}

export async function login(
  credentials: LoginCredentials,
): Promise<AuthenticationReply> {
  const { data } = await client.post<AuthenticationReply>(
    '/auth/login',
    credentials,
    anonymousRequest,
  )
  return data
}

export async function refreshSession(): Promise<AuthenticationReply> {
  const { reply } = await refreshAccessTokenSession(async () => {
    const { data } = await client.post<AuthenticationReply>(
      '/auth/refresh',
      null,
      anonymousRequest,
    )
    return data
  })
  return reply as AuthenticationReply
}

export async function logout(
  accessToken: string | null = getAccessToken(),
): Promise<void> {
  const options: KirbyRequestOptions = {
    skipAuthRefresh: true,
    skipAccessToken: true,
  }
  if (accessToken) {
    options.headers = { Authorization: `Bearer ${accessToken}` }
  }
  await client.post('/auth/logout', null, options)
}

export async function getCurrentUser(): Promise<User> {
  const { data } = await client.get<User>('/auth/me')
  return data
}
