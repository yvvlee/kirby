import { CopyOutlined, PlusOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Checkbox, Form, Input, Modal, Table, Tag } from 'antd'
import { useEffect, useState } from 'react'

import { createProjectApiKey, listProjectApiKeys, revokeProjectApiKey, rotateProjectApiKey, type ProjectApiKey } from '@/api/api-keys'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import { actionErrorMessage } from '@/features/system/errors'
import { requireKeyList, requireSecretReply } from './model'

type Props = { open: boolean; projectId: Identifier; onClose: () => void }

export default function ApiKeysPanel({ open, projectId, onClose }: Props) {
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
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      return requireKeyList(await listProjectApiKeys(environment.currentId, projectId))
    },
    enabled: open && environment.currentId !== null,
  })
  const refresh = () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.apiKeys(environment.currentId, projectId) })
  const create = useMutation({
    mutationFn: (name: string) => { if (environment.currentId === null) throw new Error('当前没有可用环境'); return createProjectApiKey(environment.currentId, projectId, name) },
  })
  const rotate = useMutation({
    mutationFn: (keyId: Identifier) => { if (environment.currentId === null) throw new Error('当前没有可用环境'); return rotateProjectApiKey(environment.currentId, projectId, keyId) },
  })
  const revoke = useMutation({
    mutationFn: (keyId: Identifier) => { if (environment.currentId === null) throw new Error('当前没有可用环境'); return revokeProjectApiKey(environment.currentId, projectId, keyId) },
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
    title: '轮换 API Key', content: `轮换“${String(key.name ?? '')}”后旧 Secret 会立即失效，是否继续？`, okText: '轮换',
    onOk: async () => { try { const reply = await rotate.mutateAsync(key.id); showSecret(reply); await refresh() } catch (error: unknown) { setActionError(error); throw error } },
  })
  const confirmRevoke = (key: ProjectApiKey) => modal.confirm({
    title: '吊销 API Key', content: `吊销“${String(key.name ?? '')}”后客户端会立即无法访问，是否继续？`, okText: '吊销', okButtonProps: { danger: true },
    onOk: async () => { try { await revoke.mutateAsync(key.id); void message.success('API Key 已吊销') } catch (error: unknown) { setActionError(error); throw error } },
  })
  const copy = async () => {
    try {
      if (!secret) throw new Error('一次性 Secret 已清除')
      if (typeof navigator.clipboard?.writeText !== 'function') throw new Error('当前浏览器不支持安全剪贴板写入')
      await navigator.clipboard.writeText(secret)
      setCopied(true)
      void message.success('Secret 已复制')
    } catch (error: unknown) {
      setActionError(error)
    }
  }
  const clearSecret = () => {
    if (!acknowledged) throw new Error('必须确认已经复制并保存 Secret')
    setSecret(null)
    setCopied(false)
    setAcknowledged(false)
  }

  const error = actionError ?? keys.error
  return (
    <Modal title="项目 API Key" open={open} width={900} footer={null} onCancel={onClose} destroyOnHidden>
      {error ? <Alert className="management-alert" type="error" showIcon message={actionErrorMessage(error, actionError ? '操作项目 API Key' : '读取项目 API Key')} /> : null}
      <header className="resource-toolbar"><p>Secret 只在创建或轮换后显示一次。</p>{canManage ? <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => { setActionError(null); form.setFieldsValue({ name: '' }); setCreateOpen(true) }}>创建 API Key</Button> : null}</header>
      <Table<ProjectApiKey> rowKey="id" loading={keys.isLoading} dataSource={keys.data ?? []} locale={{ emptyText: '当前项目还没有 API Key' }} pagination={false} scroll={{ x: 800 }} columns={[
        { title: '名称', dataIndex: 'name', width: 150 }, { title: '公开 ID', dataIndex: 'publicId', width: 190 }, { title: 'Secret 尾号', width: 120, render: (_, key) => `****${key.secretSuffix ?? ''}` }, { title: '最后使用', width: 180, render: (_, key) => key.lastUsedAt || '从未使用' }, { title: '创建时间', dataIndex: 'createdAt', width: 180 },
        ...(canManage ? [{ title: '操作', width: 150, fixed: 'right' as const, render: (_: unknown, key: ProjectApiKey) => key.revokedAt ? <Tag>已吊销</Tag> : <div className="table-actions"><Button type="link" onClick={() => confirmRotate(key)}>轮换</Button><Button type="link" danger onClick={() => confirmRevoke(key)}>吊销</Button></div> }] : []),
      ]} />
      <Modal title="创建 API Key" open={createOpen} width={480} confirmLoading={create.isPending} okText="创建" onCancel={() => { setCreateOpen(false); form.resetFields() }} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submitCreate} preserve={false}><Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入 API Key 名称' }, { max: 64, message: '名称最多 64 个字符' }]}><Input maxLength={64} placeholder="例如 production" /></Form.Item></Form>
      </Modal>
      {secret !== null ? (
        <Modal title="立即复制 API Key Secret" open width={620} closable={false} maskClosable={false} keyboard={false} footer={<Button type="primary" disabled={!acknowledged} onClick={clearSecret}>确认并清除</Button>}>
          <Alert type="warning" showIcon message="关闭后无法再次查看完整 Secret。请立即复制并安全保存。" />
          <div className="secret-value"><code>{secret}</code><Button type="primary" ghost icon={<CopyOutlined />} onClick={() => void copy()}>{copied ? '已复制' : '复制'}</Button></div>
          <Checkbox checked={acknowledged} onChange={(event) => setAcknowledged(event.target.checked)}>我确认已经复制并保存 Secret</Checkbox>
        </Modal>
      ) : null}
    </Modal>
  )
}
