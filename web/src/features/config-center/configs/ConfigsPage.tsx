import { ArrowLeftOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Form, Input, Modal, Space, Table, Tag, Tooltip } from 'antd'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { createConfig, deleteConfig } from '@/api/configs'
import { getApiErrorMessage } from '@/api/errors'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import EnvironmentTag from '@/components/EnvironmentTag/EnvironmentTag'
import ApiKeysPanel from '@/features/config-center/api-keys/ApiKeysPanel'
import { type ConfigSummary, useConfigsQuery, useProjectQuery } from '../queries'

type ConfigForm = { key: string; description: string }

function routeId(value: string | undefined, name: string): number {
  if (!value || !/^[1-9]\d*$/.test(value)) throw new TypeError(`${name} 必须是正整数`)
  return Number(value)
}

export default function ConfigsPage() {
  const { projectId: projectParam } = useParams()
  const projectId = routeId(projectParam, 'projectId')
  const navigate = useNavigate()
  const { message, modal } = App.useApp()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const [keyInput, setKeyInput] = useState('')
  const [keyFilter, setKeyFilter] = useState('')
  const [form] = Form.useForm<ConfigForm>()
  const [open, setOpen] = useState(false)
  const [apiKeysOpen, setApiKeysOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const project = useProjectQuery(environment.currentId, projectId)
  const configs = useConfigsQuery(environment.currentId, projectId, keyFilter)
  const canWrite = environment.hasPermission('config:write')
  const canReadApiKeys = environment.hasPermission('project:api_key:read') || environment.hasPermission('project:api_key:manage')
  const save = useMutation({
    mutationFn: (values: ConfigForm) => {
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      return createConfig(environment.currentId, { ...values, project_id: projectId })
    },
    onSuccess: () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.configs(environment.currentId, projectId) }),
  })
  const remove = useMutation({
    mutationFn: (configId: Identifier) => {
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      return deleteConfig(environment.currentId, configId)
    },
    onSuccess: () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.configs(environment.currentId, projectId) }),
  })

  const submit = async (values: ConfigForm) => {
    setActionError(null)
    try {
      await save.mutateAsync(values)
      setOpen(false)
      form.resetFields()
      void message.success('配置已创建')
    } catch (error: unknown) { setActionError(error) }
  }
  const confirmRemove = (config: ConfigSummary) => modal.confirm({
    title: '删除配置',
    content: `确认删除配置“${config.description || config.key}”吗？`,
    okText: '删除',
    okButtonProps: { danger: true },
    onOk: async () => {
      setActionError(null)
      try {
        await remove.mutateAsync(config.id)
        void message.success('配置已删除')
      } catch (error: unknown) {
        setActionError(error)
        throw error
      }
    },
  })

  const error = actionError ?? project.error ?? configs.error
  return (
    <section className="catalog-page" aria-labelledby="configs-title">
      <header className="detail-header">
        <Tooltip title="返回项目"><Button aria-label="返回项目" type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')} /></Tooltip>
        <div className="title-row"><h1 id="configs-title">{project.data?.name ?? '项目配置'}</h1><EnvironmentTag environment={environment.current} /></div>
      </header>
      {error ? <Alert type="error" showIcon message={getApiErrorMessage(error, '加载配置失败')} /> : null}
      <div className="catalog-toolbar">
        <Form layout="inline" onFinish={() => setKeyFilter(keyInput.trim())}>
          <Form.Item label="配置标识"><Input allowClear value={keyInput} placeholder="输入完整配置标识" onChange={(event) => setKeyInput(event.target.value)} onClear={() => setKeyFilter('')} /></Form.Item>
          <Button type="primary" htmlType="submit" loading={configs.isFetching}>搜索</Button>
        </Form>
        <Space>{canReadApiKeys ? <Button onClick={() => setApiKeysOpen(true)}>API Key</Button> : null}{canWrite ? <Button type="primary" onClick={() => { setActionError(null); form.setFieldsValue({ key: '', description: '' }); setOpen(true) }}>创建配置</Button> : null}</Space>
      </div>
      <Table<ConfigSummary>
        rowKey="id"
        loading={project.isLoading || configs.isLoading}
        dataSource={configs.data ?? []}
        locale={{ emptyText: '当前项目还没有配置' }}
        pagination={false}
        scroll={{ x: 820 }}
        columns={[
          { title: '配置标识', dataIndex: 'key', width: 190 },
          { title: '描述', dataIndex: 'description' },
          { title: '发布状态', width: 120, align: 'center', render: (_, config) => <Tag color={config.isReleased ? 'green' : 'default'}>{config.isReleased ? '已发布' : '未发布'}</Tag> },
          { title: '更新时间', dataIndex: 'updatedAt', width: 190 },
          { title: '操作', width: 190, fixed: 'right', render: (_, config) => <div className="table-actions"><Button type="link" onClick={() => navigate(`/projects/${projectId}/configs/${config.id}`)}>查看详情</Button>{canWrite ? <Button type="link" danger onClick={() => confirmRemove(config)}>删除</Button> : null}</div> },
        ]}
      />
      <Modal title="创建配置" open={open} width={540} confirmLoading={save.isPending} onCancel={() => { setOpen(false); form.resetFields() }} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <Form.Item label="配置标识" name="key" rules={[{ required: true, message: '请输入配置标识' }, { pattern: /^[A-Za-z][A-Za-z0-9]*$/, message: '配置标识只能包含字母和数字，且以字母开头' }]}><Input maxLength={64} placeholder="例如 FeatureFlags" /></Form.Item>
          <Form.Item label="配置描述" name="description"><Input.TextArea rows={3} maxLength={255} showCount /></Form.Item>
        </Form>
      </Modal>
      <ApiKeysPanel open={apiKeysOpen} projectId={projectId} onClose={() => setApiKeysOpen(false)} />
    </section>
  )
}
