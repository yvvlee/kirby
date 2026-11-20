import { useTranslation } from '@/i18n'
import { Alert, Button, Result, Spin } from 'antd'
import type { PropsWithChildren } from 'react'
import { Navigate, useLocation } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'

export function AuthGate({ children }: PropsWithChildren) {
  const { t } = useTranslation()
  const auth = useAuth()
  const location = useLocation()

  if (!auth.initialized) {
    return <div className="route-loading"><Spin size="large" /></div>
  }
  if (!auth.authenticated && auth.error) {
    return (
      <main className="status-page">
        <Result
          status="error"
          title={t('无法恢复会话')}
          subTitle={getApiErrorMessage(auth.error)}
          extra={<Button onClick={() => window.location.reload()}>{t('重新加载')}</Button>}
        />
      </main>
    )
  }
  if (!auth.authenticated) {
    const redirect = `${location.pathname}${location.search}${location.hash}`
    return <Navigate to={`/login?redirect=${encodeURIComponent(redirect)}`} replace />
  }
  return children
}

export function PermissionGate({
  permission,
  children,
}: PropsWithChildren<{ permission: string }>) {
  const { t } = useTranslation()
  const auth = useAuth()
  const environment = useEnvironment()

  if (!environment.initialized) {
    return <div className="route-loading"><Spin size="large" /></div>
  }
  if (environment.error) {
    return <Alert type="error" showIcon message={t('环境加载失败')} description={getApiErrorMessage(environment.error)} />
  }
  if (permission.startsWith('system:')) return auth.systemAdmin ? children : <Navigate to="/403" replace />
  if (environment.hasPermission(permission)) return children
  return <Navigate to="/403" replace />
}
