import client from './client'
import { positiveId } from './environment-resource'
import type { ApiListReply, ApiObject, Identifier, User } from './types'

function userPath(userId: Identifier, suffix = ''): string {
  return `/admin/users/${positiveId(userId, 'userId')}${suffix}`
}

export async function listUsers(): Promise<ApiListReply<User>> {
  const { data } = await client.get<ApiListReply<User>>('/admin/users')
  return data
}

export async function createUser(user: ApiObject): Promise<User> {
  const { data } = await client.post<User>('/admin/users', user)
  return data
}

export async function updateUser(
  userId: Identifier,
  user: ApiObject,
): Promise<User> {
  const { data } = await client.put<User>(userPath(userId), {
    ...user,
    user_id: userId,
  })
  return data
}

export async function updateUserPassword(
  userId: Identifier,
  password: string,
): Promise<ApiObject> {
  const { data } = await client.put<ApiObject>(userPath(userId, '/password'), {
    user_id: userId,
    password,
  })
  return data
}

export async function updateUserStatus(
  userId: Identifier,
  enabled: boolean,
  version: number,
): Promise<User> {
  const { data } = await client.put<User>(userPath(userId, '/status'), {
    user_id: userId,
    enabled,
    version,
  })
  return data
}
