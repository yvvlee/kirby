import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Form, Input, Modal, Switch, Table, Tag } from 'antd'
import { useState } from 'react'

import { createUser, updateUser, updateUserPassword, updateUserStatus } from '@/api/users'
import type { User } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useAuth } from '@/auth/auth-state'
import { actionErrorMessage } from './errors'
import { useUsersQuery } from './queries'

type UserForm = {
  username: string
  display_name: string
  password: string
  is_system_admin: boolean
}

export default function UsersPage() {
  const { message, modal } = App.useApp()
  const { systemAdmin } = useAuth()
  const queryClient = useQueryClient()
  const users = useUsersQuery(systemAdmin)
  const [userForm] = Form.useForm<UserForm>()
  const [passwordForm] = Form.useForm<{ password: string }>()
  const [editing, setEditing] = useState<User | null>(null)
  const [passwordUser, setPasswordUser] = useState<User | null>(null)
  const [userOpen, setUserOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const refresh = () => queryClient.invalidateQueries({ queryKey: queryKeys.users })
  const save = useMutation({
    mutationFn: (values: UserForm) => editing
      ? updateUser(editing.id, { display_name: values.display_name, is_system_admin: values.is_system_admin, version: editing.version })
      : createUser(values),
    onSuccess: refresh,
  })
  const password = useMutation({
    mutationFn: (value: string) => {
      if (!passwordUser) throw new Error('没有选择用户')
      return updateUserPassword(passwordUser.id, value)
    },
  })
  const status = useMutation({
    mutationFn: (user: User) => updateUserStatus(user.id, !user.enabled, user.version),
    onSuccess: refresh,
  })

  const closeUser = () => {
    setUserOpen(false)
    setEditing(null)
    userForm.resetFields()
  }
  const showCreate = () => {
    setActionError(null)
    setEditing(null)
    userForm.setFieldsValue({ username: '', display_name: '', password: '', is_system_admin: false })
    setUserOpen(true)
  }
  const showEdit = (user: User) => {
    setActionError(null)
    setEditing(user)
    userForm.setFieldsValue({ username: user.username, display_name: user.display_name ?? '', password: '', is_system_admin: user.is_system_admin })
    setUserOpen(true)
  }
  const submitUser = async (values: UserForm) => {
    setActionError(null)
    try {
      await save.mutateAsync(values)
      closeUser()
      void message.success('用户已保存')
    } catch (error: unknown) {
      userForm.setFieldValue('password', '')
      setActionError(error)
    }
  }
  const submitPassword = async ({ password: nextPassword }: { password: string }) => {
    setActionError(null)
    try {
      await password.mutateAsync(nextPassword)
      setPasswordOpen(false)
      setPasswordUser(null)
      passwordForm.resetFields()
      void message.success('密码已更新')
    } catch (error: unknown) {
      passwordForm.resetFields()
      setActionError(error)
    }
  }
  const confirmStatus = (user: User) => modal.confirm({
    title: '确认用户状态',
    content: `确认${user.enabled ? '停用' : '启用'}用户“${user.display_name || user.username}”吗？`,
    okText: user.enabled ? '停用' : '启用',
    onOk: async () => {
      setActionError(null)
      try {
        await status.mutateAsync(user)
        void message.success('用户状态已更新')
      } catch (error: unknown) {
        setActionError(error)
        throw error
      }
    },
  })

  const error = actionError ?? users.error
  return (
    <section className="management-panel">
      <header className="management-header">
        <div><h2>系统用户</h2><p>用户状态和系统管理员标记是全局设置。</p></div>
        {systemAdmin ? <Button type="primary" onClick={showCreate}>新建用户</Button> : null}
      </header>
      {error ? <Alert className="management-alert" type="error" showIcon message={actionErrorMessage(error, actionError ? '执行用户操作' : '读取用户')} /> : null}
      {!systemAdmin ? <Alert type="warning" showIcon message="环境管理员不能修改全局用户状态" /> : (
        <Table<User> rowKey="id" loading={users.isLoading} dataSource={users.data ?? []} locale={{ emptyText: '暂无用户' }} pagination={false} scroll={{ x: 860 }} columns={[
          { title: 'ID', dataIndex: 'id', width: 90 },
          { title: '用户名', dataIndex: 'username', width: 170 },
          { title: '显示名称', dataIndex: 'display_name', width: 170 },
          { title: '身份', width: 130, render: (_, user) => user.is_system_admin ? <Tag color="blue">系统管理员</Tag> : '普通用户' },
          { title: '状态', width: 100, render: (_, user) => <Tag color={user.enabled ? 'green' : 'default'}>{user.enabled ? '启用' : '停用'}</Tag> },
          { title: '操作', width: 230, fixed: 'right', render: (_, user) => <div className="table-actions"><Button type="link" onClick={() => showEdit(user)}>编辑</Button><Button type="link" onClick={() => { setActionError(null); setPasswordUser(user); passwordForm.resetFields(); setPasswordOpen(true) }}>改密码</Button><Button type="link" onClick={() => confirmStatus(user)}>{user.enabled ? '停用' : '启用'}</Button></div> },
        ]} />
      )}
      <Modal title={editing ? '编辑用户' : '新建用户'} open={userOpen} width={520} confirmLoading={save.isPending} onCancel={closeUser} onOk={() => userForm.submit()} afterClose={() => userForm.resetFields()} destroyOnHidden>
        <Form form={userForm} layout="vertical" onFinish={submitUser} preserve={false}>
          <Form.Item label="用户名" name="username" rules={[{ required: true, message: '请输入用户名' }, { max: 128, message: '用户名不能超过 128 个字符' }]}><Input disabled={Boolean(editing)} autoComplete="off" /></Form.Item>
          <Form.Item label="显示名称" name="display_name" rules={[{ required: true, message: '请输入显示名称' }, { max: 128, message: '显示名称不能超过 128 个字符' }]}><Input /></Form.Item>
          {!editing ? <Form.Item label="初始密码" name="password" rules={[{ required: true, message: '请输入初始密码' }, { min: 12, message: '密码至少需要 12 个字符' }]}><Input.Password autoComplete="new-password" /></Form.Item> : null}
          <Form.Item label="系统管理员" name="is_system_admin" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
      <Modal title="修改密码" open={passwordOpen} width={460} confirmLoading={password.isPending} okText="保存密码" onCancel={() => { setPasswordOpen(false); setPasswordUser(null); passwordForm.resetFields() }} onOk={() => passwordForm.submit()} afterClose={() => passwordForm.resetFields()} destroyOnHidden>
        <Form form={passwordForm} layout="vertical" onFinish={submitPassword} preserve={false}>
          <Form.Item label="新密码" name="password" rules={[{ required: true, message: '请输入新密码' }, { min: 12, message: '密码至少需要 12 个字符' }]}><Input.Password autoComplete="new-password" /></Form.Item>
        </Form>
      </Modal>
    </section>
  )
}
