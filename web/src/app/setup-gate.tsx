import { Button, Result, Spin } from 'antd'
import { type PropsWithChildren, useCallback, useEffect, useMemo, useState } from 'react'
import { Navigate, useLocation } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import { getSetupStatus, type SetupStatus } from '@/api/setup'
import { SetupContext } from './setup-state'

type InstallationState =
  | { status: 'loading' }
  | {
    status: 'ready'
    initialized: boolean
  }
  | {
    status: 'error'
    error: unknown
  }

/**
 * Resolves first-start routing before a login page or protected route is shown.
 * Parameters: children contains the application routes.
 * Returns: application routes, the setup redirect, or a retryable status error.
 * Throws: none; status failures remain visible until a successful retry.
 */
export function SetupGate({ children }: PropsWithChildren) {
  const location = useLocation()
  const [state, setState] = useState<InstallationState>({ status: 'loading' })

  const reload = useCallback(async (): Promise<SetupStatus> => {
    try {
      const status = await getSetupStatus()
      setState({
        status: 'ready',
        initialized: status.initialized,
      })
      return status
    } catch (error: unknown) {
      setState({
        status: 'error',
        error,
      })
      throw error
    }
  }, [])

  useEffect(() => {
    void reload().catch(() => undefined)
  }, [reload])

  const value = useMemo(() => ({
    initialized: state.status === 'ready' && state.initialized,
    reload,
  }), [reload, state])

  if (state.status === 'loading') {
    return <div className="route-loading"><Spin size="large" /></div>
  }
  if (state.status === 'error') {
    return (
      <Result
        status="error"
        title="无法读取初始化状态"
        subTitle={getApiErrorMessage(state.error, '请检查连接后重试')}
        extra={<Button type="primary" onClick={() => { void reload().catch(() => undefined) }}>重试</Button>}
      />
    )
  }
  if (!state.initialized && location.pathname !== '/setup') {
    return <Navigate to="/setup" replace />
  }
  return <SetupContext.Provider value={value}>{children}</SetupContext.Provider>
}
