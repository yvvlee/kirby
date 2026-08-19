import { Typography } from 'antd'
import { NavLink, Outlet } from 'react-router-dom'

import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import { actorAccess, type ActorAccess } from '@/domain/access'

const navigation: Array<{
  path: string
  label: string
  capability?: keyof ActorAccess
}> = [
  { path: '/system', label: '概览' },
  { path: '/system/environments', label: '环境', capability: 'manageEnvironments' },
  { path: '/system/users', label: '用户', capability: 'manageUsers' },
  { path: '/system/roles', label: '角色与权限', capability: 'manageRoles' },
  { path: '/system/members', label: '当前环境成员', capability: 'manageMembers' },
]

export default function SystemLayout() {
  const auth = useAuth()
  const environment = useEnvironment()
  const access = actorAccess({ systemAdmin: auth.systemAdmin, permissions: environment.permissions })
  const visibleLinks = navigation.filter((item) => !item.capability || access[item.capability])

  return (
    <section className="system-layout">
      <header className="system-header">
        <div>
          <p className="section-eyebrow">管理</p>
          <Typography.Title level={1}>环境与权限</Typography.Title>
        </div>
        <NavLink to="/">返回配置平台</NavLink>
      </header>
      <nav className="system-nav" aria-label="环境与权限管理">
        {visibleLinks.map((item) => (
          <NavLink key={item.path} to={item.path} end={item.path === '/system'}>
            {item.label}
          </NavLink>
        ))}
      </nav>
      <Outlet />
    </section>
  )
}
