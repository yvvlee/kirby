import {
  LogoutOutlined,
  ProjectOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { Alert, Button, Select, Tooltip } from 'antd'
import { useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import type { Identifier } from '@/api/types'
import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import EnvironmentTag from '@/components/EnvironmentTag/EnvironmentTag'

export default function AppLayout() {
  const auth = useAuth()
  const environment = useEnvironment()
  const location = useLocation()
  const navigate = useNavigate()
  const [failure, setFailure] = useState('')
  const showAdministration =
    auth.systemAdmin || environment.hasPermission('environment:member:manage')

  const switchEnvironment = async (environmentId: Identifier) => {
    setFailure('')
    try {
      await environment.select(environmentId)
      if (location.pathname !== '/projects') navigate('/projects', { replace: true })
    } catch (error: unknown) {
      setFailure(getApiErrorMessage(error, '环境切换失败'))
    }
  }

  const logout = async () => {
    let error: unknown = null
    try {
      await auth.logout()
    } catch (reason: unknown) {
      error = reason
    }
    navigate('/login', { replace: true })
    if (error) setFailure(`服务端退出失败：${getApiErrorMessage(error)}`)
  }

  return (
    <div className="app-shell">
      <header className="app-header">
        <Link className="app-brand" to="/">Kirby</Link>
        <nav className="app-navigation" aria-label="主导航">
          <Link to="/projects"><ProjectOutlined /> 项目</Link>
          {showAdministration ? <Link to="/system"><SettingOutlined /> 管理</Link> : null}
        </nav>
        <div className="app-actions">
          <EnvironmentTag environment={environment.current} />
          {environment.available.length ? (
            <Select
              aria-label="当前环境"
              value={environment.currentId}
              loading={environment.switching}
              options={environment.available.map((item) => ({
                label: item.name,
                value: item.id,
                disabled: !item.enabled,
              }))}
              onChange={(value) => void switchEnvironment(value)}
            />
          ) : null}
          <span className="app-user">{auth.user?.display_name || auth.user?.username}</span>
          <Tooltip title="退出">
            <Button aria-label="退出" type="text" icon={<LogoutOutlined />} onClick={() => void logout()} />
          </Tooltip>
        </div>
      </header>
      {failure ? <Alert className="shell-alert" type="error" showIcon closable message={failure} onClose={() => setFailure('')} /> : null}
      <main className="app-content"><Outlet /></main>
    </div>
  )
}
