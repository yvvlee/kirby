import { Typography } from 'antd'
import { Navigate, NavLink, Outlet } from 'react-router-dom'
import { useAuth } from '@/auth/auth-state'
import { useTranslation } from '@/i18n'
export default function SystemLayout() {
  const { t } = useTranslation()
  const auth = useAuth()
  if (!auth.systemAdmin) return <Navigate to="/403" replace />
  return <section className="system-layout">
    <header className="system-header"><Typography.Title level={1}>{t('系统管理')}</Typography.Title><NavLink to="/projects">{t('返回项目列表')}</NavLink></header>
    <nav className="system-nav" aria-label={t('系统管理')}><NavLink to="/system/users">{t('用户管理')}</NavLink><NavLink to="/system/settings">{t('系统设置')}</NavLink></nav>
    <Outlet />
  </section>
}
