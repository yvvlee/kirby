import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Empty, Form, Input, Modal, Tag } from 'antd'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { createProject } from '@/api/projects'
import { queryKeys } from '@/app/query-keys'
import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import { getApiErrorMessage } from '@/api/errors'
import { useTranslation } from '@/i18n'
import { useProjectsQuery } from '../queries'

type ProjectForm = { key: string; name: string; description: string }

export default function ProjectsPage() {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const navigate = useNavigate()
  const { user: authUser } = useAuth()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const projects = useProjectsQuery(null, keyword)
  const canWrite = Boolean(authUser)
  const [form] = Form.useForm<ProjectForm>()
  const [open, setOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const save = useMutation({
    mutationFn: (values: ProjectForm) => createProject(null, values),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.globalProjects() }),
  })

  const close = () => { setOpen(false); form.resetFields() }
  const showCreate = () => { setActionError(null); form.setFieldsValue({ key: '', name: '', description: '' }); setOpen(true) }
  const submit = async (values: ProjectForm) => {
    setActionError(null)
    try {
      await save.mutateAsync(values)
      close()
      void message.success(t('项目已创建'))
    } catch (error: unknown) { setActionError(error) }
  }

  const error = actionError ?? projects.error
  return (
    <section className="catalog-page" aria-labelledby="projects-title">
      <header className="catalog-header">
        <div><div className="title-row"><h1 id="projects-title">{t('项目')}</h1></div><p>{t('每个项目可以包含多个环境，并独立管理配置和运行时访问权限。')}</p></div>
        {canWrite ? <Button type="primary" onClick={showCreate}>{t('创建项目')}</Button> : null}
      </header>
      {error ? <Alert type="error" showIcon message={getApiErrorMessage(error, t('加载项目失败'))} /> : null}
      <Form className="filter-bar" layout="inline" onFinish={() => setKeyword(keywordInput.trim())} autoComplete="off">
        <Form.Item label={t('项目名称')}><Input allowClear value={keywordInput} placeholder={t('按名称或描述搜索')} onChange={(event) => setKeywordInput(event.target.value)} onClear={() => setKeyword('')} autoComplete="off" /></Form.Item>
        <Button type="primary" htmlType="submit" loading={projects.isFetching}>{t('搜索')}</Button>
      </Form>
      <div className="project-grid" aria-busy={projects.isFetching}>
        {(projects.data ?? []).map((project) => {
          const projectEnvironments = environment.available.filter((item) => String(item.project_id) === String(project.id))
          const openEnvironment = async (environmentId: number | string) => {
            await environment.select(environmentId)
            navigate(`/projects/${project.id}/configs`)
          }
          return (
          <article key={project.id} className="project-tile" tabIndex={0} role="link" onClick={() => {
            const first = projectEnvironments.find((item) => item.enabled)
            if (first) void openEnvironment(first.id); else navigate(`/projects/${project.id}/environments`)
          }} onKeyDown={(event) => { if (event.key === 'Enter') { const first = projectEnvironments.find((item) => item.enabled); if (first) void openEnvironment(first.id); else navigate(`/projects/${project.id}/environments`) } }}>
            <div className="project-heading"><div><h2>{project.name}</h2><code>{project.key}</code></div></div>
            <p>{project.description || t('暂无描述')}</p>
            <div className="project-environments" aria-label={t('{{name}} 的环境', { name: project.name })}>
              {projectEnvironments.map((item) => <Tag key={item.id} color={item.enabled ? 'blue' : 'default'} onClick={(event) => { event.stopPropagation(); if (item.enabled) void openEnvironment(item.id) }}>{item.name}</Tag>)}
              {!projectEnvironments.length ? <span>{t('暂无可用环境')}</span> : null}
            </div>
          </article>
          )
        })}
      </div>
      {!projects.isLoading && !projects.data?.length ? <Empty description={t('暂无项目')} /> : null}
      <Modal title={t('创建项目')} open={open} width={520} confirmLoading={save.isPending} okText={t('保存')} onCancel={close} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false} autoComplete="off">
          <Form.Item label={t('项目标识')} name="key" rules={[{ required: true, message: t('请输入项目标识') }, { pattern: /^[A-Za-z][A-Za-z0-9]*$/, message: t('项目标识只能包含字母和数字，且以字母开头') }]}><Input maxLength={64} placeholder={t('例如 DemoConfig')} autoComplete="off" /></Form.Item>
          <Form.Item label={t('项目名称')} name="name" rules={[{ required: true, message: t('请输入项目名称') }]}><Input maxLength={64} showCount autoComplete="off" /></Form.Item>
          <Form.Item label={t('项目描述')} name="description"><Input.TextArea rows={3} maxLength={255} showCount autoComplete="off" /></Form.Item>
        </Form>
      </Modal>
    </section>
  )
}
