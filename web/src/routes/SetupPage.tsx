import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Typography } from 'antd'
import axios from 'axios'
import { useState } from 'react'
import { Navigate } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import { createInitialAdministrator, type InitialAdministrator } from '@/api/setup'
import { useSetup } from '@/app/setup-state'
import { useAuth } from '@/auth/auth-state'

type SetupForm = InitialAdministrator & {
  confirm_password: string
}

function validatePassword(_rule: unknown, value?: string): Promise<void> {
  if (!value) return Promise.resolve()
  const length = new TextEncoder().encode(value).length
  if (length >= 12 && length <= 1024) return Promise.resolve()
  return Promise.reject(new Error('密码长度须为 12 至 1024 字节'))
}

/**
 * Collects the first administrator account without shipping a default password.
 * Parameters: none.
 * Returns: the setup form or the normal application entry after initialization.
 * Throws: none; account creation errors are displayed beside the form.
 */
export default function SetupPage() {
  const auth = useAuth()
  const setup = useSetup()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [created, setCreated] = useState(false)

  const submit = async ({ username, display_name, password }: SetupForm) => {
    setBusy(true)
    setError('')
    try {
      await createInitialAdministrator({
        username,
        display_name,
        password,
      })
      setCreated(true)
      await setup.reload()
    } catch (reason: unknown) {
      if (axios.isAxiosError(reason) && reason.response?.status === 409) {
        await setup.reload().catch(() => undefined)
        return
      }
      setError(getApiErrorMessage(reason, '创建管理员失败'))
    } finally {
      setBusy(false)
    }
  }

  if (setup.initialized) {
    return <Navigate to={auth.authenticated ? '/' : '/login'} replace state={{ setupComplete: created }} />
  }

  return (
    <main className="login-page" aria-labelledby="setup-title">
      <section className="login-panel">
        <p className="login-brand">Kirby</p>
        <Typography.Title id="setup-title" level={1}>创建初始管理员</Typography.Title>
        <Typography.Paragraph>设置管理员账号后，即可登录配置管理平台。</Typography.Paragraph>
        {error ? <Alert type="error" showIcon message={error} /> : null}
        <Form<SetupForm> layout="vertical" onFinish={submit} requiredMark={false}>
          <Form.Item name="username" label="用户名" rules={[{
            required: true,
            whitespace: true,
            message: '请输入用户名',
          }]}>
            <Input prefix={<UserOutlined />} autoComplete="username" autoFocus maxLength={128} />
          </Form.Item>
          <Form.Item name="display_name" label="显示名" rules={[{
            required: true,
            whitespace: true,
            message: '请输入显示名',
          }]}>
            <Input autoComplete="nickname" maxLength={128} />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[
            {
              required: true,
              message: '请输入密码',
            },
            {
              validator: validatePassword,
            },
          ]}>
            <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="confirm_password" label="确认密码" dependencies={['password']} rules={[
            {
              required: true,
              message: '请再次输入密码',
            },
            ({ getFieldValue }) => ({
              validator(_, value: string) {
                if (!value || getFieldValue('password') === value) return Promise.resolve()
                return Promise.reject(new Error('两次输入的密码不一致'))
              },
            }),
          ]}>
            <Input.Password prefix={<LockOutlined />} autoComplete="new-password" />
          </Form.Item>
          <Button block type="primary" htmlType="submit" loading={busy}>创建管理员</Button>
        </Form>
      </section>
    </main>
  )
}
