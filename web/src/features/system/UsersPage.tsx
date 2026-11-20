import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Form, Input, Modal, Switch, Table, Tag } from 'antd'
import { useState } from 'react'

import { createUser, updateUser, updateUserPassword, updateUserStatus } from '@/api/users'
import type { User } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useAuth } from '@/auth/auth-state'
import { newPasswordRules, PASSWORD_POLICY_HINT } from '@/auth/password-policy'
import { actionErrorMessage } from './errors'
import { useUsersQuery } from './queries'
import { useTranslation } from '@/i18n'

type UserForm = {
  username: string
  nickname: string
  password: string
  email: string
  is_system_admin: boolean
}

export default function UsersPage() {
  const { t } = useTranslation()
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
      ? updateUser(editing.id, { nickname: values.nickname, is_system_admin: values.is_system_admin, version: editing.version })
      : createUser(values),
    onSuccess: refresh,
  })
  const password = useMutation({
    mutationFn: (value: string) => {
      if (!passwordUser) throw new Error(t('没有选择用户'))
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
    userForm.setFieldsValue({ username: '', nickname: '', email: '', password: '', is_system_admin: false })
    setUserOpen(true)
  }
  const showEdit = (user: User) => {
    setActionError(null)
    setEditing(user)
    userForm.setFieldsValue({ username: user.username, nickname: user.nickname ?? '', password: '', is_system_admin: user.is_system_admin })
    setUserOpen(true)
  }
  const submitUser = async (values: UserForm) => {
    setActionError(null)
    try {
      await save.mutateAsync(values)
      closeUser()
      void message.success(t('用户已保存'))
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
      void message.success(t('密码已更新'))
    } catch (error: unknown) {
      passwordForm.resetFields()
      setActionError(error)
    }
  }
  const confirmStatus = (user: User) => modal.confirm({
    title: t('确认用户状态'),
    content: t('确认{{action}}用户“{{name}}”吗？', { action: t(user.enabled ? '停用用户' : '启用用户'), name: user.nickname || user.username }),
    okText: t(user.enabled ? '停用用户' : '启用用户'),
    onOk: async () => {
      setActionError(null)
      try {
        await status.mutateAsync(user)
        void message.success(t('用户状态已更新'))
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
        <div><h2>{t('系统用户')}</h2><p>{t('用户状态和系统管理员标记是全局设置。')}</p></div>
        {systemAdmin ? <Button type="primary" onClick={showCreate}>{t('新建用户')}</Button> : null}
      </header>
      {error ? <Alert className="management-alert" type="error" showIcon message={actionErrorMessage(error, t(actionError ? '执行用户操作' : '读取用户'))} /> : null}
      {!systemAdmin ? <Alert type="warning" showIcon message={t('环境管理员不能修改全局用户状态')} /> : (
        <Table<User> rowKey="id" loading={users.isLoading} dataSource={users.data ?? []} locale={{ emptyText: t('暂无用户') }} pagination={false} scroll={{ x: 860 }} columns={[
          { title: 'ID', dataIndex: 'id', width: 90 },
          { title: 'ID', dataIndex: 'id', width: 90 },
          { title: t('用户名'), dataIndex: 'username', width: 170 },
          { title: t('昵称'), dataIndex: 'nickname', width: 170 },
          { title: t('身份'), width: 130, render: (_, user) => user.is_system_admin ? <Tag color="blue">{t('系统管理员')}</Tag> : t('普通用户') },
          { title: t('状态'), width: 100, render: (_, user) => <Tag color={user.enabled ? 'green' : 'default'}>{t(user.enabled ? '启用' : '停用')}</Tag> },
          { title: t('操作'), width: 230, fixed: 'right', render: (_, user) => <div className="table-actions"><Button type="link" onClick={() => showEdit(user)}>{t('编辑')}</Button><Button type="link" onClick={() => { setActionError(null); setPasswordUser(user); passwordForm.resetFields(); setPasswordOpen(true) }}>{t('改密码')}</Button><Button type="link" onClick={() => confirmStatus(user)}>{t(user.enabled ? '停用用户' : '启用用户')}</Button></div> },
        ]} />
      )}
      <Modal title={editing ? t('编辑用户') : t('新建用户')} open={userOpen} width={520} confirmLoading={save.isPending} onCancel={closeUser} onOk={() => userForm.submit()} afterClose={() => userForm.resetFields()} destroyOnHidden>
        <Form form={userForm} layout="vertical" onFinish={submitUser} preserve={false} autoComplete="off">
          <Form.Item label={t('用户名')} name="username" rules={[{ required: true, message: t('请输入用户名') }, { max: 128, message: t('用户名不能超过 128 个字符') }]}><Input disabled={Boolean(editing)} autoComplete="off" /></Form.Item>
          <Form.Item label={t('昵称')} name="nickname" rules={[{ required: true, message: t('请输入昵称') }, { max: 128, message: t('昵称不能超过 128 个字符') }]}><Input autoComplete="off" /></Form.Item>
          {!editing ? <Form.Item label={t('邮箱')} name="email" rules={[{ required: true, message: t('请输入邮箱') }, { type: 'email', message: t('请输入有效邮箱') }]}><Input autoComplete="off" /></Form.Item> : null}
          {!editing ? <Form.Item label={t('初始密码')} name="password" rules={newPasswordRules()} extra={t(PASSWORD_POLICY_HINT)}><Input.Password autoComplete="off" /></Form.Item> : null}
          <Form.Item label={t('系统管理员')} name="is_system_admin" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
      <Modal title={t('修改密码')} open={passwordOpen} width={460} confirmLoading={password.isPending} okText={t('保存密码')} onCancel={() => { setPasswordOpen(false); setPasswordUser(null); passwordForm.resetFields() }} onOk={() => passwordForm.submit()} afterClose={() => passwordForm.resetFields()} destroyOnHidden>
        <Form form={passwordForm} layout="vertical" onFinish={submitPassword} preserve={false} autoComplete="off">
          <Form.Item label={t('新密码')} name="password" rules={newPasswordRules()} extra={t(PASSWORD_POLICY_HINT)}><Input.Password autoComplete="off" /></Form.Item>
        </Form>
      </Modal>
    </section>
  )
}
