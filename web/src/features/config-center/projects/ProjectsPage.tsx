import { EditOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Empty, Form, Input, Modal, Tooltip } from 'antd'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { createProject, updateProject } from '@/api/projects'
import { queryKeys } from '@/app/query-keys'
import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import { getApiErrorMessage } from '@/api/errors'
import { type Project, useProjectsQuery } from '../queries'

type ProjectForm = { key: string; name: string; description: string }

export default function ProjectsPage() {
  const { message } = App.useApp()
  const navigate = useNavigate()
  const { systemAdmin } = useAuth()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const projectScope = environment.currentId
  const projects = useProjectsQuery(projectScope, keyword)
  const canWrite = systemAdmin
  const [form] = Form.useForm<ProjectForm>()
  const [editing, setEditing] = useState<Project | null>(null)
  const [open, setOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const save = useMutation({
    mutationFn: (values: ProjectForm) => {
      return editing
        ? updateProject(null, { ...values, id: editing.id, version: editing.version })
        : createProject(null, values)
    },
    onSuccess: () => projectScope === null
      ? queryClient.invalidateQueries({ queryKey: queryKeys.globalProjects() })
      : queryClient.invalidateQueries({ queryKey: queryKeys.projects(projectScope) }),
  })

  const close = () => { setOpen(false); setEditing(null); form.resetFields() }
  const showCreate = () => { setActionError(null); setEditing(null); form.setFieldsValue({ key: '', name: '', description: '' }); setOpen(true) }
  const showEdit = (project: Project) => { setActionError(null); setEditing(project); form.setFieldsValue({ key: project.key, name: project.name, description: project.description ?? '' }); setOpen(true) }
  const submit = async (values: ProjectForm) => {
    setActionError(null)
    try {
      const wasEditing = Boolean(editing)
      await save.mutateAsync(values)
      close()
      void message.success(wasEditing ? '项目已更新' : '项目已创建')
    } catch (error: unknown) { setActionError(error) }
  }

  const error = actionError ?? projects.error
  return (
    <section className="catalog-page" aria-labelledby="projects-title">
      <header className="catalog-header">
        <div><div className="title-row"><h1 id="projects-title">项目</h1></div><p>每个项目可以包含多个环境，并独立管理配置和运行时访问权限。</p></div>
        {canWrite ? <Button type="primary" onClick={showCreate}>创建项目</Button> : null}
      </header>
      {error ? <Alert type="error" showIcon message={getApiErrorMessage(error, '加载项目失败')} /> : null}
      <Form className="filter-bar" layout="inline" onFinish={() => setKeyword(keywordInput.trim())}>
        <Form.Item label="项目名称"><Input allowClear value={keywordInput} placeholder="按名称或描述搜索" onChange={(event) => setKeywordInput(event.target.value)} onClear={() => setKeyword('')} /></Form.Item>
        <Button type="primary" htmlType="submit" loading={projects.isFetching}>搜索</Button>
      </Form>
      <div className="project-grid" aria-busy={projects.isFetching}>
        {(projects.data ?? []).map((project) => (
          <article key={project.id} className="project-tile" tabIndex={0} role="link" onClick={() => navigate(`/projects/${project.id}/configs`)} onKeyDown={(event) => { if (event.key === 'Enter') navigate(`/projects/${project.id}/configs`) }}>
            <div className="project-heading"><div><h2>{project.name}</h2><code>{project.key}</code></div>{canWrite ? <Tooltip title="编辑项目"><Button aria-label={`编辑项目 ${project.name}`} type="text" icon={<EditOutlined />} onClick={(event) => { event.stopPropagation(); showEdit(project) }} /></Tooltip> : null}</div>
            <p>{project.description || '暂无描述'}</p>
          </article>
        ))}
      </div>
      {!projects.isLoading && !projects.data?.length ? <Empty description="暂无项目" /> : null}
      <Modal title={editing ? '编辑项目' : '创建项目'} open={open} width={520} confirmLoading={save.isPending} okText="保存" onCancel={close} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <Form.Item label="项目标识" name="key" rules={[{ required: true, message: '请输入项目标识' }, { pattern: /^[A-Za-z][A-Za-z0-9]*$/, message: '项目标识只能包含字母和数字，且以字母开头' }]}><Input disabled={Boolean(editing)} maxLength={64} placeholder="例如 DemoConfig" /></Form.Item>
          <Form.Item label="项目名称" name="name" rules={[{ required: true, message: '请输入项目名称' }]}><Input maxLength={64} showCount /></Form.Item>
          <Form.Item label="项目描述" name="description"><Input.TextArea rows={3} maxLength={255} showCount /></Form.Item>
        </Form>
      </Modal>
    </section>
  )
}
