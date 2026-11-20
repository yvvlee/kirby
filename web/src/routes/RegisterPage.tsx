import { LockOutlined, MailOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'

import { getApiErrorMessage } from '@/api/errors'
import { newPasswordRules, PASSWORD_POLICY_HINT } from '@/auth/password-policy'
import { safeRedirect } from '@/app/routing'
import { useTranslation } from '@/i18n'
import LanguageSwitcher from '@/i18n/LanguageSwitcher'
import {
  getRegistrationStatus,
  register,
  type RegistrationRequest,
} from '@/api/auth'

export default function RegisterPage() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const redirect = safeRedirect(params.get('redirect'))
  const [canRegister, setCanRegister] = useState(false)
  useEffect(() => {
    void getRegistrationStatus()
      .then(setCanRegister)
      .catch((reason) => setError(reason))
  }, [])
  const [error, setError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)
  const submit = async (
    request: RegistrationRequest & { confirmPassword: string },
  ) => {
    setError(null)
    setBusy(true)
    try {
      await register({
        username: request.username,
        nickname: request.nickname,
        email: request.email,
        password: request.password,
      })
      window.location.assign(redirect)
      return
    } catch (reason: unknown) {
      setError(reason)
    } finally {
      setBusy(false)
    }
  }
  return (
    <main className="login-page" aria-labelledby="register-title">
      <section className="login-panel">
        <LanguageSwitcher />
        <p className="login-brand">Kirby</p>
        <Typography.Title id="register-title" level={1}>
          {t('注册配置管理平台')}
        </Typography.Title>
        {error ? <Alert type="error" showIcon message={getApiErrorMessage(error, t('注册失败'))} /> : null}
        {canRegister ? (
          <Form layout="vertical" onFinish={submit} requiredMark={false} autoComplete="off">
            <Form.Item
              name="username"
              label={t('用户名')}
              rules={[{ required: true }]}
            >
              <Input prefix={<UserOutlined />} autoComplete="off" />
            </Form.Item>
            <Form.Item
              name="nickname"
              label={t('昵称')}
              rules={[{ required: true }]}
            >
              <Input autoComplete="off" />
            </Form.Item>
            <Form.Item
              name="email"
              label={t('邮箱')}
              rules={[{ required: true, type: 'email' }]}
            >
              <Input prefix={<MailOutlined />} autoComplete="off" />
            </Form.Item>
            <Form.Item
              name="password"
              label={t('密码')}
              rules={newPasswordRules()}
              extra={t(PASSWORD_POLICY_HINT)}
            >
              <Input.Password
                prefix={<LockOutlined />}
                autoComplete="off"
              />
            </Form.Item>
            <Form.Item
              name="confirmPassword"
              label={t('确认密码')}
              dependencies={['password']}
              rules={[
                { required: true },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    return value === getFieldValue('password')
                      ? Promise.resolve()
                      : Promise.reject(new Error(t('两次密码不一致')))
                  },
                }),
              ]}
            >
              <Input.Password autoComplete="off" />
            </Form.Item>
            <Button block type="primary" htmlType="submit" loading={busy}>
              {t('注册')}
            </Button>
          </Form>
        ) : (
          <Alert type="info" message={t('系统暂未开放自行注册')} />
        )}
        <Typography.Paragraph>
          <Link to={`/login?redirect=${encodeURIComponent(redirect)}`}>
            {t('返回登录')}
          </Link>
        </Typography.Paragraph>
      </section>
    </main>
  )
}
