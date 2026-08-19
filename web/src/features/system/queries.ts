import { useQuery } from '@tanstack/react-query'

import { listRoles, listPermissions, listEnvironmentMembers } from '@/api/roles'
import { listUsers } from '@/api/users'
import type { ApiEntity, Identifier, User } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { requireList } from './errors'

export type Permission = ApiEntity & {
  key: string
  name: string
  description?: string
}

export type Role = ApiEntity & {
  key: string
  name: string
  description?: string
  version: number
  builtin?: boolean
  permissions?: Permission[]
}

export type EnvironmentMember = {
  user: User
  roles?: Role[]
}

export function useUsersQuery(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.users,
    queryFn: async () => requireList<User>(await listUsers(), 'user list'),
    enabled,
  })
}

export function useRolesQuery(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.roles,
    queryFn: async () => requireList<Role>(await listRoles(), 'role list'),
    enabled,
  })
}

export function usePermissionsQuery(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.permissions,
    queryFn: async () => requireList<Permission>(await listPermissions(), 'permission list'),
    enabled,
  })
}

export function useEnvironmentMembersQuery(
  environmentId: Identifier | null,
  enabled: boolean,
) {
  return useQuery({
    queryKey: environmentId === null
      ? ['environment', 'none', 'members']
      : queryKeys.environmentMembers(environmentId),
    queryFn: async () => {
      if (environmentId === null) throw new Error('请先选择环境')
      return requireList<EnvironmentMember>(
        await listEnvironmentMembers(environmentId),
        'environment member list',
      )
    },
    enabled: enabled && environmentId !== null,
  })
}
