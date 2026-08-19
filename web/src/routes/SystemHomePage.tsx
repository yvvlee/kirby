import { Alert, Typography } from 'antd'

import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import { actorAccess } from '@/domain/access'

export default function SystemHomePage() {
  const auth = useAuth()
  const environment = useEnvironment()
  const access = actorAccess({ systemAdmin: auth.systemAdmin, permissions: environment.permissions })
  const availableCount = Object.values(access).filter(Boolean).length
  return (
    <section className="content-section">
      <Typography.Title level={2}>可用管理功能</Typography.Title>
      {availableCount ? (
        <Typography.Text type="secondary">当前账号可以使用 {availableCount} 项管理功能。</Typography.Text>
      ) : (
        <Alert type="info" showIcon message="当前账号没有环境或系统管理权限" />
      )}
    </section>
  )
}
