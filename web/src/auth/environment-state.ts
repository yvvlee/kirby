import { createContext, useContext } from 'react'

import type { Environment, Identifier } from '@/api/types'

export type EnvironmentContextValue = {
  available: Environment[]
  current: Environment | null
  currentId: Identifier | null
  permissions: string[]
  switching: boolean
  initialized: boolean
  error: unknown
  hasPermission: (permission: string) => boolean
  loadAvailable: () => Promise<Environment[]>
  select: (environmentId: Identifier) => Promise<Environment | null>
  resetScope: () => Promise<void>
}

export const EnvironmentContext =
  createContext<EnvironmentContextValue | null>(null)

export function useEnvironment(): EnvironmentContextValue {
  const value = useContext(EnvironmentContext)
  if (!value) {
    throw new Error('useEnvironment must be used inside EnvironmentProvider')
  }
  return value
}
