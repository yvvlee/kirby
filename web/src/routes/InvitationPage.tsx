import { Alert, Button, Card, Space, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import client, { type KirbyRequestOptions } from '@/api/client'
import { getApiErrorMessage } from '@/api/errors'
import { getRegistrationStatus } from '@/api/auth'
import { useQueryClient } from '@tanstack/react-query'
import { useEnvironment } from '@/auth/environment-state'
import { useAuth } from '@/auth/auth-state'
import { useTranslation } from '@/i18n'
import LanguageSwitcher from '@/i18n/LanguageSwitcher'
import { projectRoleLabel } from '@/i18n/project-roles'

type Invitation = {
  project_id: string
  role: string
  expires_at: string
}

export default function InvitationPage() {
  const { t } = useTranslation()
  const { token = '' } = useParams()
  const auth = useAuth()
  const environment = useEnvironment()
  const cache = useQueryClient()
  const navigate = useNavigate()
  const [invitation, setInvitation] = useState<Invitation | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [canRegister, setCanRegister] = useState(false)
  const path = `/invitations/${encodeURIComponent(token)}`
  const redirect = encodeURIComponent(path)
  useEffect(() => {
    void client
      .get<Invitation>(path, {
        skipAccessToken: true,
        skipAuthRefresh: true,
      } as KirbyRequestOptions)
      .then(({ data }) => setInvitation(data))
      .catch((reason: unknown) =>
        setError(getApiErrorMessage(reason, t('邀请链接无效或已过期'))),
      )
    void getRegistrationStatus()
      .then(setCanRegister)
      .catch(() => setCanRegister(false))
  }, [path, t])
  const accept = async () => {
    setBusy(true)
    try {
      await client.post(`${path}/accept`, { token })
      await environment.loadAvailable()
      await cache.invalidateQueries()
      await navigate('/projects')
    } catch (reason: unknown) {
      setError(getApiErrorMessage(reason, t('加入失败')))
    } finally {
      setBusy(false)
    }
  }
  return (
    <main className="login-page">
      <LanguageSwitcher />
      <Card title={t('项目邀请')}>
        {error ? <Alert type="error" message={error} /> : null}
        {invitation ? (
          <Space direction="vertical">
            <Typography.Paragraph>
              {t('你将以 {{role}} 身份加入项目 {{projectId}}。', { role: projectRoleLabel(invitation.role), projectId: invitation.project_id })}
            </Typography.Paragraph>
            <Typography.Text>{t('有效期至 {{expiresAt}}', { expiresAt: invitation.expires_at })}</Typography.Text>
            {auth.authenticated ? (
              <Button
                type="primary"
                loading={busy}
                onClick={() => void accept()}
              >
                {t('确认加入项目')}
              </Button>
            ) : (
              <Space>
                <Link to={`/login?redirect=${redirect}`}>{t('登录后确认')}</Link>
                {canRegister ? (
                  <Link to={`/register?redirect=${redirect}`}>{t('注册后确认')}</Link>
                ) : null}
              </Space>
            )}
          </Space>
        ) : null}
      </Card>
    </main>
  )
}
