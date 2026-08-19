import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Spin, Typography } from 'antd'
import { useState } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import type { LoginCredentials } from '@/api/auth'
import { safeRedirect } from '@/app/routing'
import { useAuth } from '@/auth/auth-state'

export default function LoginPage() {
  const auth = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [error, setError] = useState('')
  const redirect = safeRedirect(searchParams.get('redirect'))

  if (!auth.initialized) return <div className="route-loading"><Spin size="large" /></div>
  if (auth.authenticated) return <Navigate to={redirect} replace />

  const submit = async (credentials: LoginCredentials) => {
    setError('')
    try {
      await auth.login(credentials)
      await navigate(redirect, { replace: true })
    } catch (reason: unknown) {
      setError(getApiErrorMessage(reason, '登录失败'))
    }
  }

  return (
    <main className="login-page" aria-labelledby="login-title">
      <section className="login-panel">
        <p className="login-brand">Kirby</p>
        <Typography.Title id="login-title" level={1}>登录配置管理平台</Typography.Title>
        {error ? <Alert type="error" showIcon message={error} /> : null}
        <Form<LoginCredentials> layout="vertical" onFinish={submit} requiredMark={false}>
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} autoComplete="username" autoFocus />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} autoComplete="current-password" />
          </Form.Item>
          <Button block type="primary" htmlType="submit" loading={auth.busy}>登录</Button>
        </Form>
      </section>
    </main>
  )
}
