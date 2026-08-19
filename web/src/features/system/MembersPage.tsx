import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Checkbox, Form, Input, Modal, Table, Tag } from 'antd'
import { useMemo, useState } from 'react'

import { updateEnvironmentMemberRoles } from '@/api/roles'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import { actorAccess, assignableRoles } from '@/domain/access'
import { actionErrorMessage } from './errors'
import { type EnvironmentMember, useEnvironmentMembersQuery, useRolesQuery } from './queries'

type MemberForm = { userId: string; roleIds: Identifier[] }

export default function MembersPage() {
  const { message } = App.useApp()
  const auth = useAuth()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const access = actorAccess({ systemAdmin: auth.systemAdmin, permissions: environment.permissions })
  const canManage = access.manageMembers
  const members = useEnvironmentMembersQuery(environment.currentId, canManage)
  const roles = useRolesQuery(canManage)
  const availableRoles = useMemo(() => assignableRoles(roles.data ?? [], auth.systemAdmin), [auth.systemAdmin, roles.data])
  const [form] = Form.useForm<MemberForm>()
  const [editing, setEditing] = useState<EnvironmentMember | null>(null)
  const [open, setOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const save = useMutation({
    mutationFn: ({ userId, roleIds }: MemberForm) => {
      if (environment.currentId === null) throw new Error('请先选择环境')
      return updateEnvironmentMemberRoles(environment.currentId, userId, roleIds)
    },
    onSuccess: () => environment.currentId === null
      ? Promise.resolve()
      : queryClient.invalidateQueries({ queryKey: queryKeys.environmentMembers(environment.currentId) }),
  })

  const showAdd = () => {
    setActionError(null)
    setEditing(null)
    form.setFieldsValue({ userId: '', roleIds: [] })
    setOpen(true)
  }
  const showEdit = (member: EnvironmentMember) => {
    setActionError(null)
    setEditing(member)
    form.setFieldsValue({ userId: String(member.user.id), roleIds: (member.roles ?? []).map((role) => role.id) })
    setOpen(true)
  }
  const close = () => {
    setOpen(false)
    setEditing(null)
    form.resetFields()
  }
  const submit = async (values: MemberForm) => {
    setActionError(null)
    try {
      await save.mutateAsync(values)
      close()
      void message.success('环境成员角色已更新')
    } catch (error: unknown) {
      setActionError(error)
    }
  }

  const queryError = members.error ?? roles.error
  return (
    <section className="management-panel">
      <header className="management-header">
        <div>
          <h2>当前环境成员</h2>
          <p>{environment.current ? `只修改“${environment.current.name}”中的成员角色。` : '请先选择一个可用环境。'}</p>
        </div>
        {canManage && environment.current ? <Button type="primary" onClick={showAdd}>添加成员</Button> : null}
      </header>
      {actionError ?? queryError ? <Alert className="management-alert" type="error" showIcon message={actionErrorMessage(actionError ?? queryError, actionError ? '保存环境成员角色' : '读取环境成员')} /> : null}
      {!canManage ? <Alert type="warning" showIcon message="当前账号没有管理此环境成员的权限" /> : (
        <Table<EnvironmentMember>
          rowKey={(member) => member.user.id}
          loading={members.isLoading || roles.isLoading}
          dataSource={members.data ?? []}
          locale={{ emptyText: '当前环境暂无成员' }}
          pagination={false}
          scroll={{ x: 650 }}
          columns={[
            { title: '用户', width: 220, render: (_, member) => <div><strong>{member.user.display_name || member.user.username}</strong><p className="secondary-text">{member.user.username}</p></div> },
            { title: '当前环境角色', render: (_, member) => member.roles?.length ? member.roles.map((role) => <Tag key={role.id}>{role.name}</Tag>) : '未分配' },
            { title: '操作', width: 120, fixed: 'right', render: (_, member) => <Button type="link" onClick={() => showEdit(member)}>分配角色</Button> },
          ]}
        />
      )}
      <Modal title={editing ? '分配环境角色' : '添加环境成员'} open={open} width={560} confirmLoading={save.isPending} okButtonProps={{ disabled: availableRoles.length === 0 }} okText="保存角色" onCancel={close} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <Form.Item label="用户 ID" name="userId" rules={[{ required: true, message: '请输入用户 ID' }, { pattern: /^[1-9]\d*$/, message: '用户 ID 必须是正整数' }]} extra="环境管理员不能读取或修改全局用户资料。请填写已有用户 ID。"><Input disabled={Boolean(editing)} inputMode="numeric" placeholder="输入已有系统用户的 ID" /></Form.Item>
          <Form.Item label="环境角色" name="roleIds">
            <Checkbox.Group className="role-list">
              {availableRoles.map((role) => <Checkbox key={role.id} value={role.id}>{role.name}（{role.key}）</Checkbox>)}
            </Checkbox.Group>
          </Form.Item>
          {availableRoles.length === 0 ? <p className="secondary-text">后端没有返回可分配角色。</p> : null}
        </Form>
      </Modal>
    </section>
  )
}
