import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Spin, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { Link, Navigate, useLocation, useNavigate, useSearchParams } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import { getRegistrationStatus, type LoginCredentials } from '@/api/auth'
import { safeRedirect } from '@/app/routing'
import { useAuth } from '@/auth/auth-state'
import { useTranslation } from '@/i18n'
import LanguageSwitcher from '@/i18n/LanguageSwitcher'

export default function LoginPage() {
  const { t } = useTranslation()
  const auth = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams] = useSearchParams()
  const [error, setError] = useState<unknown>(null)
  const [canRegister, setCanRegister] = useState(false)
  useEffect(() => {
    void getRegistrationStatus().then(setCanRegister).catch((reason: unknown) => {
      setError(reason)
    })
  }, [])
  const redirect = safeRedirect(searchParams.get('redirect'))

  if (!auth.initialized) return <div className="route-loading"><Spin size="large" /></div>
  if (auth.authenticated) return <Navigate to={redirect} replace />

  const submit = async (credentials: LoginCredentials) => {
    setError(null)
    try {
      await auth.login(credentials)
      await navigate(redirect, { replace: true })
    } catch (reason: unknown) {
      setError(reason)
    }
  }

  return (
    <main className="login-page" aria-labelledby="login-title">
      <section className="login-panel">
        <LanguageSwitcher />
        <p className="login-brand">Kirby</p>
        <Typography.Title id="login-title" level={1}>{t('登录配置管理平台')}</Typography.Title>
        {location.state?.setupComplete ? <Alert type="success" showIcon message={t('管理员已创建，请登录')} /> : null}
        {error ? <Alert type="error" showIcon message={getApiErrorMessage(error, t('登录失败'))} /> : null}
        <Form<LoginCredentials> layout="vertical" onFinish={submit} requiredMark={false} autoComplete="off">
          <Form.Item name="identifier" label={t('用户名或邮箱')} rules={[{ required: true, message: t('请输入用户名或邮箱') }]}>
            <Input prefix={<UserOutlined />} autoComplete="off" autoFocus />
          </Form.Item>
          <Form.Item name="password" label={t('密码')} rules={[{ required: true, message: t('请输入密码') }]}>
            <Input.Password prefix={<LockOutlined />} autoComplete="off" />
          </Form.Item>
          <Button block type="primary" htmlType="submit" loading={auth.busy}>{t('登录')}</Button>
        </Form>
        <Typography.Paragraph>{canRegister ? <Link to={`/register?redirect=${encodeURIComponent(redirect)}`}>{t('注册新账号')}</Link> : null}</Typography.Paragraph>
      </section>
    </main>
  )
}
