import { useQueryClient } from '@tanstack/react-query'
import {
  type PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'

import {
  getMyPermissions,
  listEnvironments,
} from '@/api/environments'
import type { Environment, Identifier } from '@/api/types'
import { isEnvironmentQuery, queryKeys } from '@/app/query-keys'
import { clearEnvironmentScope } from './environment-scope'
import { getAccessTokenSnapshot } from './token'
import { useAuth } from './auth-state'
import {
  EnvironmentContext,
  type EnvironmentContextValue,
} from './environment-state'

function sameId(left: Identifier | null, right: Identifier | null): boolean {
  return left !== null && right !== null && String(left) === String(right)
}

function requireEnvironmentList(value: unknown): Environment[] {
  if (
    typeof value !== 'object' ||
    value === null ||
    !('list' in value) ||
    !Array.isArray(value.list)
  ) {
    throw new TypeError('environment list response does not contain list')
  }
  return value.list as Environment[]
}

function requirePermissions(value: unknown): string[] {
  if (
    typeof value !== 'object' ||
    value === null ||
    !('permissions' in value) ||
    !Array.isArray(value.permissions) ||
    !value.permissions.every((item) => typeof item === 'string')
  ) {
    throw new TypeError('permission response does not contain permissions')
  }
  return value.permissions
}

export function EnvironmentProvider({ children }: PropsWithChildren) {
  const { authenticated } = useAuth()
  const queryClient = useQueryClient()
  const [available, setAvailable] = useState<Environment[]>([])
  const [currentId, setCurrentId] = useState<Identifier | null>(null)
  const [permissions, setPermissions] = useState<string[]>([])
  const [switching, setSwitching] = useState(false)
  const [initialized, setInitialized] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const availableRef = useRef(available)
  const currentIdRef = useRef(currentId)
  const switchSequence = useRef(0)
  const lifecycleVersion = useRef(0)

  useEffect(() => {
    availableRef.current = available
  }, [available])
  useEffect(() => {
    currentIdRef.current = currentId
  }, [currentId])

  const resetScope = useCallback(async () => {
    lifecycleVersion.current += 1
    switchSequence.current += 1
    const previousId = currentIdRef.current
    setSwitching(false)
    setInitialized(true)
    setError(null)
    setCurrentId(null)
    setPermissions([])
    setAvailable([])
    availableRef.current = []
    currentIdRef.current = null
    await queryClient.cancelQueries({ predicate: (query) => isEnvironmentQuery(query.queryKey) })
    queryClient.removeQueries({ predicate: (query) => isEnvironmentQuery(query.queryKey) })
    if (previousId !== null) {
      await clearEnvironmentScope({
        fromEnvironmentId: previousId,
        toEnvironmentId: null,
      })
    }
  }, [queryClient])

  const select = useCallback(
    async (environmentId: Identifier): Promise<Environment | null> => {
      const environment = availableRef.current.find((item) =>
        sameId(item.id, environmentId),
      )
      if (!environment) {
        throw new Error(`environment is not available: ${environmentId}`)
      }
      if (!environment.enabled) {
        throw new Error(`environment is disabled: ${environmentId}`)
      }

      switchSequence.current += 1
      const sequence = switchSequence.current
      const lifecycle = lifecycleVersion.current
      const sessionGeneration = getAccessTokenSnapshot().sessionGeneration
      setSwitching(true)
      setError(null)
      try {
        const reply = await queryClient.fetchQuery({
          queryKey: queryKeys.environmentPermissions(environment.id),
          queryFn: () => getMyPermissions(environment.id),
        })
        const nextPermissions = requirePermissions(reply)
        if (
          sequence !== switchSequence.current ||
          lifecycle !== lifecycleVersion.current ||
          sessionGeneration !== getAccessTokenSnapshot().sessionGeneration
        ) {
          queryClient.removeQueries({
            queryKey: queryKeys.environment(environment.id),
          })
          return null
        }

        const previousId = currentIdRef.current
        const otherEnvironment = (queryKey: readonly unknown[]) =>
          isEnvironmentQuery(queryKey) &&
          queryKey[1] !== String(environment.id)
        await queryClient.cancelQueries({
          predicate: (query) =>
            otherEnvironment(query.queryKey) &&
            query.queryKey[2] !== 'permissions',
        })
        queryClient.removeQueries({
          predicate: (query) =>
            otherEnvironment(query.queryKey) &&
            query.state.fetchStatus !== 'fetching',
        })
        if (previousId !== null && !sameId(previousId, environment.id)) {
          await clearEnvironmentScope({
            fromEnvironmentId: previousId,
            toEnvironmentId: environment.id,
          })
        }
        if (
          sequence !== switchSequence.current ||
          lifecycle !== lifecycleVersion.current ||
          sessionGeneration !== getAccessTokenSnapshot().sessionGeneration
        ) {
          return null
        }

        currentIdRef.current = environment.id
        setCurrentId(environment.id)
        setPermissions(nextPermissions)
        return environment
      } catch (error: unknown) {
        setError(error)
        throw error
      } finally {
        if (sequence === switchSequence.current) setSwitching(false)
      }
    },
    [queryClient],
  )

  const loadAvailable = useCallback(async (): Promise<Environment[]> => {
    const lifecycle = lifecycleVersion.current
    const sessionGeneration = getAccessTokenSnapshot().sessionGeneration
    const reply = await queryClient.fetchQuery({
      queryKey: queryKeys.environments,
      queryFn: listEnvironments,
    })
    const environments = requireEnvironmentList(reply)
    if (
      lifecycle !== lifecycleVersion.current ||
      sessionGeneration !== getAccessTokenSnapshot().sessionGeneration
    ) {
      return []
    }
    availableRef.current = environments
    setAvailable(environments)

    const current = environments.find(
      (item) => item.enabled && sameId(item.id, currentIdRef.current),
    )
    const next = current ?? environments.find((item) => item.enabled)
    if (next) await select(next.id)
    return environments
  }, [queryClient, select])

  useEffect(() => {
    if (authenticated) {
      setInitialized(false)
      void loadAvailable()
        .then(() => setInitialized(true))
        .catch((reason: unknown) => {
          setError(reason)
          setInitialized(true)
        })
    } else {
      void resetScope()
    }
  }, [authenticated, loadAvailable, resetScope])

  const current = useMemo(
    () => available.find((item) => sameId(item.id, currentId)) ?? null,
    [available, currentId],
  )
  const value = useMemo<EnvironmentContextValue>(
    () => ({
      available,
      current,
      currentId,
      permissions,
      switching,
      initialized,
      error,
      hasPermission: (permission) => permissions.includes(permission),
      loadAvailable,
      select,
      resetScope,
    }),
    [available, current, currentId, error, initialized, loadAvailable, permissions, resetScope, select, switching],
  )

  return (
    <EnvironmentContext.Provider value={value}>
      {children}
    </EnvironmentContext.Provider>
  )
}
