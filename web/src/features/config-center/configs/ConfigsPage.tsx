import ProjectDangerZone from '../projects/ProjectDangerZone'
import { ArrowLeftOutlined, SettingOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query'
import { Alert, App, Button, Dropdown, Form, Input, Modal, Select, Space, Table, Tag, Tooltip } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { listProjectMembers } from '@/api/project-members'
import { useAuth } from '@/auth/auth-state'
import { createConfig, deleteConfig } from '@/api/configs'
import { updateProject } from '@/api/projects'
import { getApiErrorMessage } from '@/api/errors'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import { useTranslation } from '@/i18n'
import { localizedError } from '@/i18n/errors'
import ApiKeysPanel from '@/features/config-center/api-keys/ApiKeysPanel'
import { type ConfigSummary, useConfigsQuery, useProjectQuery } from '../queries'

type ConfigForm = { key: string; description: string }
type ProjectForm = { name: string; description: string }

function routeId(value: string | undefined, name: string): number {
  if (!value || !/^[1-9]\d*$/.test(value)) throw localizedError('{{name}} 必须是正整数', { name }, TypeError)
  return Number(value)
}

export default function ConfigsPage() {
  const { t } = useTranslation()
  const { projectId: projectParam } = useParams()
  const projectId = routeId(projectParam, 'projectId')
  const navigate = useNavigate()
  const auth = useAuth()
  const members = useQuery({ queryKey: ['project-members', String(projectId)], queryFn: () => listProjectMembers(projectId) })
  const projectRole = members.data?.list.find((member) => String(member.user.id) === String(auth.user?.id))?.role
  const isOwner = projectRole === 'OWNER'
  const canManage = isOwner || projectRole === 'MAINTAINER'
  const { message, modal } = App.useApp()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const [keyInput, setKeyInput] = useState('')
  const [keyFilter, setKeyFilter] = useState('')
  const [form] = Form.useForm<ConfigForm>()
  const [open, setOpen] = useState(false)
  const [apiKeysOpen, setApiKeysOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const [projectSettingsOpen, setProjectSettingsOpen] = useState(false)
  const [projectForm] = Form.useForm<ProjectForm>()
  const project = useProjectQuery(environment.currentId, projectId)
  const projectEnvironments = useMemo(() => environment.available.filter((item) => String(item.project_id) === String(projectId)), [environment.available, projectId])
  useEffect(() => {
    const currentBelongsToProject = projectEnvironments.some((item) => String(item.id) === String(environment.currentId))
    const next = currentBelongsToProject ? null : projectEnvironments.find((item) => item.enabled)
    if (next) void environment.select(next.id)
  }, [environment, environment.currentId, projectEnvironments])
  const canRead = environment.hasPermission('config:read')
  const configs = useConfigsQuery(canRead ? environment.currentId : null, projectId, keyFilter)
  const canWrite = environment.hasPermission('config:write')
  const canReadApiKeys = environment.hasPermission('project:api_key:read') || environment.hasPermission('project:api_key:manage')
  const save = useMutation({
    mutationFn: (values: ConfigForm) => {
      if (environment.currentId === null) throw localizedError('当前没有可用环境')
      return createConfig(environment.currentId, { ...values, project_id: projectId })
    },
    onSuccess: () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.configs(environment.currentId, projectId) }),
  })
  const updateProjectMutation = useMutation({
    mutationFn: (values: ProjectForm) => updateProject(null, {
      ...values,
      id: projectId,
      version: project.data?.version,
    }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.project(environment.currentId ?? 0, projectId) }),
  })
  const remove = useMutation({
    mutationFn: (configId: Identifier) => {
      if (environment.currentId === null) throw localizedError('当前没有可用环境')
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
      void message.success(t('配置已创建'))
    } catch (error: unknown) { setActionError(error) }
  }
  const confirmRemove = (config: ConfigSummary) => modal.confirm({
    title: t('删除配置'),
    content: t('确认删除配置“{{name}}”吗？', { name: config.description || config.key }),
    okText: t('删除'),
    okButtonProps: { danger: true },
    onOk: async () => {
      setActionError(null)
      try {
        await remove.mutateAsync(config.id)
        void message.success(t('配置已删除'))
      } catch (error: unknown) {
        setActionError(error)
        throw error
      }
    },
  })

  const openProjectSettings = () => {
    if (!project.data) return
    projectForm.setFieldsValue({
      name: project.data.name,
      description: project.data.description ?? '',
    })
    setProjectSettingsOpen(true)
  }
  const projectMenuItems = [
    {
      key: 'members',
      label: t('项目成员'),
      onClick: () => navigate(`/projects/${projectId}/members`),
    },
    {
      key: 'permissions',
      label: t('环境权限'),
      disabled: !environment.currentId,
      onClick: () => navigate(`/projects/${projectId}/environments/${environment.currentId}/permissions`),
    },
    ...(isOwner ? [
      {
        key: 'environments',
        label: t('环境管理'),
        onClick: () => navigate(`/projects/${projectId}/environments`),
      },
      {
        key: 'settings',
        label: t('项目设置'),
        disabled: !project.data,
        onClick: openProjectSettings,
      },
    ] : []),
  ]

  const error = actionError ?? project.error ?? configs.error
  return (
    <section className="catalog-page" aria-labelledby="configs-title">
      <header className="detail-header">
        <Tooltip title={t('返回项目')}><Button aria-label={t('返回项目')} type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')} /></Tooltip>
        <div className="title-row"><h1 id="configs-title">{project.data?.name ?? t('项目配置')}</h1></div>
        <div className="project-header-actions">
          {projectEnvironments.length ? (
            <div className="project-environment-selector">
              <label htmlFor="project-environment">{t('当前环境：')}</label>
              <Select
                id="project-environment"
                aria-label={t('项目环境')}
                value={environment.currentId}
                loading={environment.switching}
                options={projectEnvironments.map((item) => ({
                  label: item.name,
                  value: item.id,
                  disabled: !item.enabled,
                }))}
                onChange={(value) => void environment.select(value)}
              />
            </div>
          ) : null}
          {canManage ? (
            <Dropdown menu={{ items: projectMenuItems }} trigger={['click']} placement="bottomRight">
              <Button aria-label={t('项目管理菜单')} title={t('项目管理')} type="text" icon={<SettingOutlined />} />
            </Dropdown>
          ) : null}
        </div>
      </header>
      {error ? <Alert type="error" showIcon message={getApiErrorMessage(error, t('加载配置失败'))} /> : null}
      <div className="catalog-toolbar">
        <Form layout="inline" onFinish={() => setKeyFilter(keyInput.trim())} autoComplete="off">
          <Form.Item label={t('配置标识')}><Input allowClear value={keyInput} placeholder={t('输入完整配置标识')} onChange={(event) => setKeyInput(event.target.value)} onClear={() => setKeyFilter('')} autoComplete="off" /></Form.Item>
          <Button type="primary" htmlType="submit" loading={configs.isFetching}>{t('搜索')}</Button>
        </Form>
        <Space>{canReadApiKeys ? <Button onClick={() => setApiKeysOpen(true)}>API Key</Button> : null}{canWrite ? <Button type="primary" onClick={() => { setActionError(null); form.setFieldsValue({ key: '', description: '' }); setOpen(true) }}>{t('创建配置')}</Button> : null}</Space>
      </div>
      {canRead ? <Table<ConfigSummary>
        rowKey="id"
        loading={project.isLoading || configs.isLoading}
        dataSource={configs.data ?? []}
        locale={{ emptyText: t('当前项目还没有配置') }}
        pagination={false}
        scroll={{ x: 820 }}
        columns={[
          { title: t('配置标识'), dataIndex: 'key', width: 190 },
          { title: t('描述'), dataIndex: 'description' },
          { title: t('发布状态'), width: 120, align: 'center', render: (_, config) => <Tag color={config.isReleased ? 'green' : 'default'}>{config.isReleased ? t('已发布') : t('未发布')}</Tag> },
          { title: t('更新时间'), dataIndex: 'updatedAt', width: 190 },
          { title: t('操作'), width: 190, fixed: 'right', render: (_, config) => <div className="table-actions"><Button type="link" onClick={() => navigate(`/projects/${projectId}/configs/${config.id}`)}>{t('查看详情')}</Button>{canWrite ? <Button type="link" danger onClick={() => confirmRemove(config)}>{t('删除')}</Button> : null}</div> },
        ]}
      /> : <Alert type="info" message={t('当前环境未授予配置读取权限')} />}
      <Modal title={t('创建配置')} open={open} width={540} confirmLoading={save.isPending} onCancel={() => { setOpen(false); form.resetFields() }} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false} autoComplete="off">
          <Form.Item label={t('配置标识')} name="key" rules={[{ required: true, message: t('请输入配置标识') }, { pattern: /^[A-Za-z][A-Za-z0-9]*$/, message: t('配置标识只能包含字母和数字，且以字母开头') }]}><Input maxLength={64} placeholder={t('例如 FeatureFlags')} autoComplete="off" /></Form.Item>
          <Form.Item label={t('配置描述')} name="description"><Input.TextArea rows={3} maxLength={255} showCount autoComplete="off" /></Form.Item>
        </Form>
      </Modal>
      <Modal title={t('项目设置')} open={isOwner && projectSettingsOpen} confirmLoading={updateProjectMutation.isPending} okText={t('保存')} onCancel={() => setProjectSettingsOpen(false)} onOk={() => projectForm.submit()} destroyOnHidden>
        <Form form={projectForm} layout="vertical" onFinish={async (values) => { await updateProjectMutation.mutateAsync(values); setProjectSettingsOpen(false); void message.success(t('项目已更新')) }} autoComplete="off">
          <Form.Item label={t('项目名称')} name="name" rules={[{ required: true, message: t('请输入项目名称') }]}><Input autoComplete="off" /></Form.Item>
          <Form.Item label={t('项目描述')} name="description"><Input.TextArea rows={3} autoComplete="off" /></Form.Item>
        </Form>
        {project.data ? <ProjectDangerZone projectId={projectId} projectName={project.data.name} /> : null}
      </Modal>
      <ApiKeysPanel open={apiKeysOpen} projectId={projectId} onClose={() => setApiKeysOpen(false)} />
    </section>
  )
}
