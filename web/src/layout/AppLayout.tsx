import { useTranslation } from '@/i18n'
import LanguageSwitcher from '@/i18n/LanguageSwitcher'
import {
  LogoutOutlined,
  SettingOutlined,
} from '@ant-design/icons'
import { Alert, Button, Tooltip } from 'antd'
import { useState } from 'react'
import { Link, Outlet, useNavigate } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import { useAuth } from '@/auth/auth-state'

export default function AppLayout() {
  const { t } = useTranslation()
  const auth = useAuth()
  const navigate = useNavigate()
  const [failure, setFailure] = useState('')
  const showAdministration = auth.systemAdmin

  const logout = async () => {
    let error: unknown = null
    try {
      await auth.logout()
    } catch (reason: unknown) {
      error = reason
    }
    navigate('/login', { replace: true })
    if (error) setFailure(t('服务端退出失败：{{error}}', { error: getApiErrorMessage(error) }))
  }

  return (
    <div className="app-shell">
      <header className="app-header">
        <Link className="app-brand" to="/">Kirby</Link>
        <div className="app-actions">
          <LanguageSwitcher />
          {showAdministration ? <Tooltip title={t('全局管理')}><Button aria-label={t('全局管理')} type="text" icon={<SettingOutlined />} onClick={() => navigate('/system')} /></Tooltip> : null}
          <span className="app-user">{auth.user?.nickname || auth.user?.username}</span>
          <Tooltip title={t('退出')}>
            <Button aria-label={t('退出')} type="text" icon={<LogoutOutlined />} onClick={() => void logout()} />
          </Tooltip>
        </div>
      </header>
      {failure ? <Alert className="shell-alert" type="error" showIcon closable message={failure} onClose={() => setFailure('')} /> : null}
      <main className="app-content"><Outlet /></main>
    </div>
  )
}
