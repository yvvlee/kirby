import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Typography } from 'antd'
import axios from 'axios'
import { useState } from 'react'
import { Navigate } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import { createInitialAdministrator, type InitialAdministrator } from '@/api/setup'
import { useSetup } from '@/app/setup-state'
import { useAuth } from '@/auth/auth-state'
import { newPasswordRules, PASSWORD_POLICY_HINT } from '@/auth/password-policy'
import { useTranslation } from '@/i18n'
import LanguageSwitcher from '@/i18n/LanguageSwitcher'

type SetupForm = InitialAdministrator & {
  confirm_password: string
}

/**
 * Collects the first administrator account without shipping a default password.
 * Parameters: none.
 * Returns: the setup form or the normal application entry after initialization.
 * Throws: none; account creation errors are displayed beside the form.
 */
export default function SetupPage() {
  const { t } = useTranslation()
  const auth = useAuth()
  const setup = useSetup()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [created, setCreated] = useState(false)

  const submit = async ({ username, nickname, password, email }: SetupForm) => {
    setBusy(true)
    setError(null)
    try {
      await createInitialAdministrator({
        username,
        email,
        nickname,
        password,
      })
      setCreated(true)
      await setup.reload()
    } catch (reason: unknown) {
      if (axios.isAxiosError(reason) && reason.response?.status === 409) {
        await setup.reload().catch(() => undefined)
        return
      }
      setError(reason)
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
        <LanguageSwitcher />
        <p className="login-brand">Kirby</p>
        <Typography.Title id="setup-title" level={1}>{t('创建初始管理员')}</Typography.Title>
        <Typography.Paragraph>{t('设置管理员账号后，即可登录配置管理平台。')}</Typography.Paragraph>
        {error ? <Alert type="error" showIcon message={getApiErrorMessage(error, t('创建管理员失败'))} /> : null}
        <Form<SetupForm> layout="vertical" onFinish={submit} requiredMark={false} autoComplete="off">
          <Form.Item name="username" label={t('用户名')} rules={[{
            required: true,
            whitespace: true,
            message: t('请输入用户名'),
          }]}>
            <Input prefix={<UserOutlined />} autoComplete="off" autoFocus maxLength={128} />
          </Form.Item>
          <Form.Item name="nickname" label={t('昵称')} rules={[{
            required: true,
            whitespace: true,
            message: t('请输入昵称'),
          }]}>
            <Input autoComplete="off" maxLength={128} />
          </Form.Item>
          <Form.Item name="email" label={t('邮箱')} rules={[{required:true,type:"email", message: t('请输入有效邮箱')}]}><Input autoComplete="off" /></Form.Item>
          <Form.Item name="password" label={t('密码')} rules={newPasswordRules()} extra={t(PASSWORD_POLICY_HINT)}>
            <Input.Password prefix={<LockOutlined />} autoComplete="off" />
          </Form.Item>
          <Form.Item name="confirm_password" label={t('确认密码')} dependencies={['password']} rules={[
            {
              required: true,
              message: t('请再次输入密码'),
            },
            ({ getFieldValue }) => ({
              validator(_, value: string) {
                if (!value || getFieldValue('password') === value) return Promise.resolve()
                return Promise.reject(new Error(t('两次输入的密码不一致')))
              },
            }),
          ]}>
            <Input.Password prefix={<LockOutlined />} autoComplete="off" />
          </Form.Item>
          <Button block type="primary" htmlType="submit" loading={busy}>{t('创建管理员')}</Button>
        </Form>
      </section>
    </main>
  )
}
