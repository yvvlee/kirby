import { createContext, useContext } from 'react'

import type { SetupStatus } from '@/api/setup'

export type SetupContextValue = {
  initialized: boolean
  reload: () => Promise<SetupStatus>
}

export const SetupContext = createContext<SetupContextValue | null>(null)

/**
 * Shares installation state so account creation can refresh the routing decision.
 * Parameters: none.
 * Returns: the installation status and its reload operation.
 * Throws: when the page is mounted outside SetupGate.
 */
export function useSetup(): SetupContextValue {
  const value = useContext(SetupContext)
  if (!value) throw new Error('useSetup must be used inside SetupGate')
  return value
}
