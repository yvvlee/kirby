import { CopyOutlined, PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Checkbox, Form, Input, Modal, Table, Tag } from 'antd'
import { useEffect, useState } from 'react'

import { createProjectApiKey, listProjectApiKeys, revokeProjectApiKey, rotateProjectApiKey, type ProjectApiKey } from '@/api/api-keys'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import { useTranslation } from '@/i18n'
import { localizedError } from '@/i18n/errors'
import { actionErrorMessage } from '@/features/system/errors'
import { requireKeyList, requireSecretReply } from './model'

type Props = { open: boolean; projectId: Identifier; onClose: () => void }

function formatTimestamp(value: string, locale: string): string {
  return new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).format(new Date(value)).replaceAll('/', '-')
}

export default function ApiKeysPanel({ open, projectId, onClose }: Props) {
  const { t, locale } = useTranslation()
  const { message, modal } = App.useApp()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const [form] = Form.useForm<{ name: string }>()
  const [createOpen, setCreateOpen] = useState(false)
  const [secret, setSecret] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [acknowledged, setAcknowledged] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const canManage = environment.hasPermission('project:api_key:manage')
  const keys = useQuery({
    queryKey: environment.currentId === null ? ['environment', 'none', 'api-keys'] : queryKeys.apiKeys(environment.currentId, projectId),
    queryFn: async () => {
      if (environment.currentId === null) throw localizedError('当前没有可用环境')
      return requireKeyList(await listProjectApiKeys(environment.currentId, projectId))
    },
    enabled: open && environment.currentId !== null,
  })
  const refresh = () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.apiKeys(environment.currentId, projectId) })
  const create = useMutation({
    mutationFn: (name: string) => { if (environment.currentId === null) throw localizedError('当前没有可用环境'); return createProjectApiKey(environment.currentId, projectId, name) },
  })
  const rotate = useMutation({
    mutationFn: (keyId: Identifier) => { if (environment.currentId === null) throw localizedError('当前没有可用环境'); return rotateProjectApiKey(environment.currentId, projectId, keyId) },
  })
  const revoke = useMutation({
    mutationFn: (keyId: Identifier) => { if (environment.currentId === null) throw localizedError('当前没有可用环境'); return revokeProjectApiKey(environment.currentId, projectId, keyId) },
    onSuccess: refresh,
  })

  useEffect(() => {
    if (!open) {
      setSecret(null)
      setCopied(false)
      setAcknowledged(false)
      setCreateOpen(false)
      form.resetFields()
    }
  }, [form, open])

  const showSecret = (reply: Awaited<ReturnType<typeof createProjectApiKey>>) => {
    setSecret(requireSecretReply(reply))
    setCopied(false)
    setAcknowledged(false)
  }
  const submitCreate = async ({ name }: { name: string }) => {
    setActionError(null)
    try {
      const reply = await create.mutateAsync(name)
      setCreateOpen(false)
      form.resetFields()
      showSecret(reply)
      await refresh()
    } catch (error: unknown) { setActionError(error) }
  }
  const confirmRotate = (key: ProjectApiKey) => modal.confirm({
    title: t('轮换 API Key'), content: t('轮换“{{name}}”后旧 Secret 会立即失效，是否继续？', { name: String(key.name ?? '') }), okText: t('轮换'),
    onOk: async () => { try { const reply = await rotate.mutateAsync(key.id); showSecret(reply); await refresh() } catch (error: unknown) { setActionError(error); throw error } },
  })
  const confirmRevoke = (key: ProjectApiKey) => modal.confirm({
    title: t('吊销 API Key'), content: t('吊销“{{name}}”后客户端会立即无法访问，是否继续？', { name: String(key.name ?? '') }), okText: t('吊销'), okButtonProps: { danger: true },
    onOk: async () => { try { await revoke.mutateAsync(key.id); void message.success(t('API Key 已吊销')) } catch (error: unknown) { setActionError(error); throw error } },
  })
  const copy = async () => {
    try {
      if (!secret) throw localizedError('一次性 Secret 已清除')
      if (typeof navigator.clipboard?.writeText !== 'function') throw localizedError('当前浏览器不支持安全剪贴板写入')
      await navigator.clipboard.writeText(secret)
      setCopied(true)
      void message.success(t('Secret 已复制'))
    } catch (error: unknown) {
      setActionError(error)
    }
  }
  const clearSecret = () => {
    if (!acknowledged) throw localizedError('必须确认已经复制并保存 Secret')
    setSecret(null)
    setCopied(false)
    setAcknowledged(false)
  }

  const error = actionError ?? keys.error
  return (
    <Modal title={<span className="api-keys-title"><span>API Key</span><span>{t('环境：{{name}}', { name: environment.current?.name ?? '' })}</span></span>} open={open} width={1000} footer={null} onCancel={onClose} destroyOnHidden>
      {error ? <Alert className="management-alert" type="error" showIcon message={actionErrorMessage(error, t(actionError ? '操作项目 API Key' : '读取项目 API Key'))} /> : null}
      <header className="resource-toolbar api-keys-toolbar">{canManage ? <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => { setActionError(null); form.setFieldsValue({ name: '' }); setCreateOpen(true) }}>{t('创建 API Key')}</Button> : null}</header>
      <Table<ProjectApiKey>
        className="api-keys-table"
        tableLayout="fixed"
        rowKey="id"
        loading={keys.isLoading}
        dataSource={keys.data ?? []}
        locale={{ emptyText: t('当前环境还没有 API Key') }}
        pagination={false}
        columns={[
          {
            title: t('名称'),
            dataIndex: 'name',
            width: '20%',
          },
          {
            title: t('Secret 尾号'),
            width: '16%',
            render: (_, key) => `****${key.secretSuffix ?? ''}`,
          },
          {
            title: t('最后使用'),
            width: '24%',
            render: (_, key) => key.lastUsedAt ? formatTimestamp(key.lastUsedAt, locale) : t('从未使用'),
          },
          {
            title: t('创建时间'),
            width: '24%',
            render: (_, key) => key.createdAt ? formatTimestamp(key.createdAt, locale) : '—',
          },
          ...(canManage ? [{
            title: t('操作'),
            width: '16%',
            render: (_: unknown, key: ProjectApiKey) => key.revokedAt ? <Tag>{t('已吊销')}</Tag> : <div className="table-actions"><Button type="link" onClick={() => confirmRotate(key)}>{t('轮换')}</Button><Button type="link" danger onClick={() => confirmRevoke(key)}>{t('吊销')}</Button></div>,
          }] : []),
        ]}
      />
      <Modal title={t('创建 API Key')} open={createOpen} width={480} confirmLoading={create.isPending} okText={t('创建')} onCancel={() => { setCreateOpen(false); form.resetFields() }} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submitCreate} preserve={false} autoComplete="off"><Form.Item label={t('名称')} name="name" rules={[{ required: true, message: t('请输入 API Key 名称') }, { max: 64, message: t('名称最多 64 个字符') }]}><Input maxLength={64} placeholder={t('例如 production')} autoComplete="off" /></Form.Item></Form>
      </Modal>
      {secret !== null ? (
        <Modal title={t('立即复制 API Key Secret')} open width={620} closable={false} maskClosable={false} keyboard={false} footer={<Button type="primary" disabled={!acknowledged} onClick={clearSecret}>{t('确认并清除')}</Button>}>
          <Alert type="warning" showIcon message={t('关闭后无法再次查看完整 Secret。请立即复制并安全保存。')} />
          <div className="secret-value"><code>{secret}</code><Button type="primary" ghost icon={<CopyOutlined />} onClick={() => void copy()}>{copied ? t('已复制') : t('复制')}</Button></div>
          <Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>{t('我确认已经复制并保存 Secret')}</Checkbox>
        </Modal>
      ) : null}
    </Modal>
  )
}
