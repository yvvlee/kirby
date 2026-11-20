import { refreshAccessTokenSession } from '@/auth/refresh-coordinator'
import { getAccessToken } from '@/auth/token'

import client, { type KirbyRequestOptions } from './client'
import type { ApiObject, User } from './types'

export type LoginCredentials = {
  identifier?: string
  username?: string
  password: string
}

export type AuthenticationReply = ApiObject & {
  access_token: string
  user: User
}

export type RegistrationRequest = {
	username: string
	nickname: string
	email: string
	password: string
}

export async function register(
	request: RegistrationRequest,
): Promise<AuthenticationReply> {
	const { data } = await client.post<AuthenticationReply>('/auth/register', request, anonymousRequest)
	return data
}

const anonymousRequest: KirbyRequestOptions = {
  skipAuthRefresh: true,
  skipAccessToken: true,
}

export async function login(
  credentials: LoginCredentials,
): Promise<AuthenticationReply> {
  const identifier = credentials.identifier ?? credentials.username ?? ''
  const { data } = await client.post<AuthenticationReply>(
    '/auth/login',
    { username: identifier, password: credentials.password },
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

export async function getRegistrationStatus(): Promise<boolean> {
 const {data} = await client.get<{allow_self_registration: boolean}>('/auth/registration-status', anonymousRequest)
 return data.allow_self_registration
}
