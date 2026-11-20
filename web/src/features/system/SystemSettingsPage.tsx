import { Alert, Button, Form, Switch } from 'antd'
import { useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import client from '@/api/client'
import { getApiErrorMessage } from '@/api/errors'
import { useTranslation } from '@/i18n'

type Settings = {
  allow_self_registration: boolean
}
export default function SystemSettingsPage() {
  const { t } = useTranslation()
  const [form] = Form.useForm<Settings>()
  const cache = useQueryClient()
  const settings = useQuery({ queryKey: ['system-settings'], queryFn: async () => (await client.get<Settings>('/admin/system-settings')).data })
  const save = useMutation({ mutationFn: async (values: Settings) => client.put('/admin/system-settings', values), onSuccess: () => cache.invalidateQueries({ queryKey: ['system-settings'] }) })
  useEffect(() => { if (settings.data) form.setFieldsValue(settings.data) }, [settings.data, form])
  const error = settings.error ?? save.error
  return <section className="management-panel"><h2>{t('系统设置')}</h2>
    {error ? <Alert type="error" message={getApiErrorMessage(error, t('系统设置操作失败'))} /> : null}
    <Form form={form} layout="vertical" onFinish={(values) => save.mutate(values)} autoComplete="off">
      <Form.Item name="allow_self_registration" label={t('允许自行注册')} valuePropName="checked" extra={t('关闭时只能由管理员创建账号。开启后，项目管理员也可以生成邀请链接。')}><Switch disabled={settings.isLoading} /></Form.Item>
      <Button type="primary" htmlType="submit" loading={save.isPending} disabled={!settings.data}>{t('保存设置')}</Button>
      {save.isSuccess ? <p role="status">{t('设置已保存')}</p> : null}
    </Form>
  </section>
}
