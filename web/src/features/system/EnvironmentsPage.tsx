import { useMutation, useQuery } from '@tanstack/react-query'
import {
  Alert,
  App,
  Button,
  Form,
  Input,
  Modal,
  Switch,
  Table,
  Tag,
} from 'antd'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { listProjectMembers } from '@/api/project-members'
import { createEnvironment, updateEnvironment } from '@/api/environments'
import type { Environment } from '@/api/types'
import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import { useProjectsQuery } from '@/features/config-center/queries'
import { actionErrorMessage } from './errors'
import { useTranslation } from '@/i18n'
type EnvironmentForm = {
  project_id: number
  key: string
  name: string
  description: string
  enabled: boolean
}
export default function EnvironmentsPage() {
  const { t } = useTranslation()
  const { message } = App.useApp()
  const { user } = useAuth()
  const environmentState = useEnvironment()
  const { projectId: projectParam } = useParams()
  const projectId = Number(projectParam)
  const navigate = useNavigate()
  if (!Number.isSafeInteger(projectId) || projectId <= 0)
    throw new Error(t('项目标识无效'))
  const members = useQuery({
    queryKey: ['project-members', String(projectId)],
    queryFn: () => listProjectMembers(projectId),
  })
  const isOwner = members.data?.list.some(
    (member) =>
      String(member.user.id) === String(user?.id) && member.role === 'OWNER',
  )
  const projects = useProjectsQuery(null, '')
  const [form] = Form.useForm<EnvironmentForm>()
  const [editing, setEditing] = useState<Environment | null>(null)
  const [open, setOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const save = useMutation({
    mutationFn: async (values: EnvironmentForm) =>
      editing
        ? updateEnvironment(editing.id, {
            ...values,
            version: editing.version,
          })
        : createEnvironment({
            ...values,
            project_id: projectId,
          }),
    onSuccess: () => environmentState.loadAvailable(),
  })
  const close = () => {
    setOpen(false)
    setEditing(null)
    form.resetFields()
  }
  const showCreate = () => {
    setActionError(null)
    setEditing(null)
    form.setFieldsValue({
      project_id: projectId,
      key: '',
      name: '',
      description: '',
      enabled: true,
    })
    setOpen(true)
  }
  const showEdit = (item: Environment) => {
    setActionError(null)
    setEditing(item)
    form.setFieldsValue({
      project_id: Number(item.project_id),
      key: item.key,
      name: item.name,
      description: item.description ?? '',
      enabled: item.enabled,
    })
    setOpen(true)
  }
  const submit = async (values: EnvironmentForm) => {
    setActionError(null)
    try {
      await save.mutateAsync(values)
      close()
      void message.success(t('环境已保存'))
    } catch (error: unknown) {
      setActionError(error)
    }
  }
  const openProjectConfigs = async () => {
    const first = environmentState.available.find(
      (item) => String(item.project_id) === String(projectId) && item.enabled,
    )!
    await environmentState.select(first.id)
    navigate(`/projects/${projectId}/configs`)
  }
  const error = actionError ?? environmentState.error
  return (
    <section className="management-panel">
      <header className="management-header">
        <div>
          <Link to="/projects">{t('返回项目列表')}</Link>
          <h2>
            {projects.data?.find(
              (project) => String(project.id) === String(projectId),
            )?.name ?? t('项目')}{' '}
            / {t('环境')}
          </h2>
          <p>{t('环境属于当前项目，用于隔离配置数据和 API Key。')}</p>
        </div>
        <div>
          {environmentState.available.some(
            (item) =>
              String(item.project_id) === String(projectId) && item.enabled,
          ) ? (
            <Button onClick={() => void openProjectConfigs()}>{t('项目配置')}</Button>
          ) : null}
          {isOwner ? (
            <Button type="primary" onClick={showCreate}>
              {t('新建环境')}
            </Button>
          ) : null}
        </div>
      </header>
      {error ? (
        <Alert
          className="management-alert"
          type="error"
          showIcon
          message={actionErrorMessage(error, t('读取环境'))}
        />
      ) : null}
      {!isOwner ? (
        <Alert type="warning" showIcon message={t('只有项目 Owner 可以修改环境')} />
      ) : (
        <Table<Environment>
          rowKey="id"
          loading={!environmentState.initialized || environmentState.switching}
          dataSource={environmentState.available.filter(
            (item) => String(item.project_id) === String(projectId),
          )}
          locale={{ emptyText: t('暂无环境') }}
          pagination={false}
          scroll={{ x: 760 }}
          columns={[
            {
              title: t('名称'),
              dataIndex: 'name',
              width: 180,
            },
            {
              title: t('标识'),
              dataIndex: 'key',
              width: 160,
            },
            {
              title: t('说明'),
              dataIndex: 'description',
            },
            {
              title: t('状态'),
              width: 100,
              render: (_, item) => (
                <Tag color={item.enabled ? 'green' : 'default'}>
                  {t(item.enabled ? '启用' : '停用')}
                </Tag>
              ),
            },
            {
              title: t('操作'),
              width: 90,
              fixed: 'right',
              render: (_, item) => (
                <Button type="link" onClick={() => showEdit(item)}>
                  {t('编辑')}
                </Button>
              ),
            },
          ]}
        />
      )}
      <Modal
        title={editing ? t('编辑环境') : t('新建环境')}
        open={open}
        width={520}
        confirmLoading={save.isPending}
        okText={t('保存')}
        onCancel={close}
        onOk={() => form.submit()}
        afterClose={() => form.resetFields()}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={submit} preserve={false} autoComplete="off">
          <Form.Item
            label={t('环境标识')}
            name="key"
            rules={[
              {
                required: true,
                message: t('请输入环境标识'),
              },
              {
                pattern: /^[a-z][a-z0-9-]*$/,
                message: t('只能使用小写字母、数字和连字符，且必须以字母开头'),
              },
            ]}
          >
            <Input disabled={Boolean(editing)} placeholder={t('例如 production')} autoComplete="off" />
          </Form.Item>
          <Form.Item
            label={t('名称')}
            name="name"
            rules={[
              {
                required: true,
                message: t('请输入环境名称'),
              },
            ]}
          >
            <Input autoComplete="off" />
          </Form.Item>
          <Form.Item
            label={t('说明')}
            name="description"
            rules={[
              {
                max: 255,
                message: t('说明不能超过 255 个字符'),
              },
            ]}
          >
            <Input.TextArea rows={3} autoComplete="off" />
          </Form.Item>
          {editing ? (
            <Form.Item label={t('状态')} name="enabled" valuePropName="checked">
              <Switch checkedChildren={t('启用')} unCheckedChildren={t('停用')} />
            </Form.Item>
          ) : null}
        </Form>
      </Modal>
    </section>
  )
}
