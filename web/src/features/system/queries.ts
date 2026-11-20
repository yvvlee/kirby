import { useQuery } from '@tanstack/react-query'
import { listUsers } from '@/api/users'
import type { User } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { requireList } from './errors'

export function useUsersQuery(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.users,
    queryFn: async () => requireList<User>(await listUsers(), 'user list'),
    enabled,
  })
}
