import {
  Alert,
  Button,
  Form,
  Breadcrumb,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { useState } from 'react'
import { useAuth } from '@/auth/auth-state'
import { getApiErrorMessage } from '@/api/errors'
import {
  addProjectMember,
  listProjectMembers,
  removeProjectMember,
  searchProjectUsers,
  updateProjectMemberRole,
  type ProjectMember,
} from '@/api/project-members'
import { getProject } from '@/api/projects'
import { normalizeProjectDetail } from '../queries'
import { getRegistrationStatus } from '@/api/auth'
import {
  createProjectInvitation,
  revokeProjectInvitation,
} from '@/api/project-invitations'
import { useTranslation } from '@/i18n'
import { projectRoleLabel } from '@/i18n/project-roles'
const roles = ['OWNER', 'MAINTAINER', 'MEMBER', 'GUEST']
export default function ProjectMembersPage() {
  const { t } = useTranslation()
  const { projectId = '' } = useParams()
  const project = useQuery({
    queryKey: ['project-detail', projectId],
    queryFn: async () => normalizeProjectDetail(await getProject(null, projectId)),
  })
  const auth = useAuth()
  const cache = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [inviteRole, setInviteRole] = useState('GUEST')
  const [form] = Form.useForm()
  const queryKey = ['project-members', projectId]
  const members = useQuery({
    queryKey,
    queryFn: () => listProjectMembers(projectId),
  })
  const ownRole = members.data?.list.find(
    (member) => String(member.user.id) === String(auth.user?.id),
  )?.role
  const canManage = ownRole === 'OWNER' || ownRole === 'MAINTAINER'
  const options = roles
    .filter((role) => roles.indexOf(role) >= roles.indexOf(ownRole ?? 'GUEST'))
    .map((value) => ({
      value,
      label: projectRoleLabel(value),
    }))
  const users = useQuery({
    queryKey: ['project-user-search', projectId, keyword],
    queryFn: () => searchProjectUsers(projectId, keyword),
    enabled: canManage,
  })
  const change = useMutation({
    mutationFn: async (action: () => Promise<unknown>) => action(),
    onSuccess: () => cache.invalidateQueries({ queryKey }),
  })
  const editable = (member: ProjectMember) =>
    canManage && roles.indexOf(member.role) >= roles.indexOf(ownRole ?? 'GUEST')
  const registration = useQuery({
    queryKey: ['registration-status'],
    queryFn: getRegistrationStatus,
  })
  const invitation = useMutation({
    mutationFn: () => createProjectInvitation(projectId, inviteRole),
  })
  const revoke = useMutation({
    mutationFn: (token: string) => revokeProjectInvitation(token),
    onSuccess: () => invitation.reset(),
  })
  const error =
    project.error ??
    invitation.error ??
    revoke.error ??
    registration.error ??
    members.error ??
    users.error ??
    change.error
  return (
    <section className="catalog-page">
      <Breadcrumb items={[
        { title: <Link to="/projects">{t('项目')}</Link> },
        { title: <Link to={`/projects/${projectId}/configs`}>{project.data?.name ?? t('加载中…')}</Link> },
        { title: t('项目成员') },
      ]} />
      <h1>{t('项目成员')}</h1>
      {error ? (
        <Alert
          type="error"
          message={getApiErrorMessage(error, t('成员操作失败'))}
        />
      ) : null}
      {canManage ? (
        <Form
          form={form}
          layout="inline"
          initialValues={{ role: 'GUEST' }}
          onFinish={(values) =>
            change.mutate(() =>
              addProjectMember(projectId, values.userId, values.role),
            )
          } autoComplete="off"
        >
          <Form.Item
            label={t('添加用户')}
            name="userId"
            rules={[
              {
                required: true,
                message: t('请选择用户'),
              },
            ]}
          >
            <Select
              style={{ width: 360 }}
              placeholder={t('搜索用户名、昵称或邮箱')}
              aria-label={t('选择已有用户')}
              showSearch
              filterOption={false}
              onSearch={setKeyword}
              onSelect={() => setKeyword('')}
              notFoundContent={users.isFetching ? t('加载中…') : t('没有可添加的用户')}
              loading={users.isFetching}
              options={(users.data?.list ?? []).filter((user) => !members.data?.list.some((member) => String(member.user.id) === String(user.id))).map((user) => ({
                value: user.id,
                label: `${user.nickname ?? user.username} (@${user.username}) ${user.email ?? ''}`,
              }))}
            />
          </Form.Item>
          <Form.Item name="role">
            <Select style={{ width: 160 }} options={options} />
          </Form.Item>
          <Button htmlType="submit" type="primary" loading={change.isPending}>
            {t('添加成员')}
          </Button>
        </Form>
      ) : null}
      {canManage && registration.data ? (
        <Space direction="vertical" style={{ marginBlock: 20 }}>
          <Space>
            <Select
              aria-label={t('邀请角色')}
              value={inviteRole}
              onChange={setInviteRole}
              options={options}
              style={{ width: 160 }}
            />
            <Button
              loading={invitation.isPending}
              onClick={() => invitation.mutate()}
            >
              {t('生成邀请链接')}
            </Button>
          </Space>
          {invitation.data ? (
            <Space direction="vertical">
              <Typography.Paragraph
                copyable
              >{`${window.location.origin}/invitations/${invitation.data.token}`}</Typography.Paragraph>
              <Typography.Text>
                {t('此链接仅可使用一次，登录或注册后需确认加入。有效期至 {{expiresAt}}。', { expiresAt: invitation.data.expires_at })}
              </Typography.Text>
              <Button
                danger
                loading={revoke.isPending}
                onClick={() => revoke.mutate(invitation.data.token)}
              >
                {t('撤销链接')}
              </Button>
            </Space>
          ) : null}
        </Space>
      ) : null}
      <Table<ProjectMember>
        rowKey={(member) => String(member.user.id)}
        loading={members.isLoading}
        dataSource={members.data?.list ?? []}
        pagination={false}
        columns={[
          {
            title: t('昵称'),
            render: (_, member) => member.user.nickname ?? member.user.username,
          },
          {
            title: t('用户名'),
            render: (_, member) => member.user.username,
          },
          {
            title: t('角色'),
            render: (_, member) =>
              editable(member) ? (
                <Select
                  value={member.role}
                  options={options}
                  disabled={change.isPending}
                  style={{ width: 160 }}
                  onChange={(role) =>
                    change.mutate(() =>
                      updateProjectMemberRole(projectId, member.user.id, role),
                    )
                  }
                />
              ) : (
                <Tag>{projectRoleLabel(member.role)}</Tag>
              ),
          },
          {
            title: t('操作'),
            render: (_, member) =>
              editable(member) ? (
                <Space>
                  <Popconfirm
                    title={t('确认移除此成员？')}
                    onConfirm={() =>
                      change.mutateAsync(() =>
                        removeProjectMember(projectId, member.user.id),
                      )
                    }
                  >
                    <Button danger disabled={change.isPending}>
                      {t('移除')}
                    </Button>
                  </Popconfirm>
                </Space>
              ) : null,
          },
        ]}
      />
    </section>
  )
}
