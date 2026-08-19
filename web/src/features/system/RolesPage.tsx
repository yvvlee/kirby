import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Checkbox, Form, Input, Modal, Table, Tag } from 'antd'
import { useState } from 'react'

import { createRole, deleteRole, updateRole, updateRolePermissions } from '@/api/roles'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useAuth } from '@/auth/auth-state'
import { actionErrorMessage } from './errors'
import { type Role, usePermissionsQuery, useRolesQuery } from './queries'

type RoleForm = { key: string; name: string; description: string }

export default function RolesPage() {
  const { message, modal } = App.useApp()
  const { systemAdmin } = useAuth()
  const queryClient = useQueryClient()
  const roles = useRolesQuery(systemAdmin)
  const permissions = usePermissionsQuery(systemAdmin)
  const [form] = Form.useForm<RoleForm>()
  const [editing, setEditing] = useState<Role | null>(null)
  const [permissionRole, setPermissionRole] = useState<Role | null>(null)
  const [selectedPermissionIds, setSelectedPermissionIds] = useState<Identifier[]>([])
  const [roleOpen, setRoleOpen] = useState(false)
  const [permissionOpen, setPermissionOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const refreshRoles = () => queryClient.invalidateQueries({ queryKey: queryKeys.roles })
  const save = useMutation({
    mutationFn: (values: RoleForm) => editing
      ? updateRole(editing.id, { name: values.name, description: values.description, version: editing.version })
      : createRole(values),
    onSuccess: refreshRoles,
  })
  const savePermissions = useMutation({
    mutationFn: () => {
      if (!permissionRole) throw new Error('没有选择角色')
      return updateRolePermissions(permissionRole.id, selectedPermissionIds)
    },
    onSuccess: refreshRoles,
  })
  const remove = useMutation({ mutationFn: deleteRole, onSuccess: refreshRoles })

  const closeRole = () => {
    setRoleOpen(false)
    setEditing(null)
    form.resetFields()
  }
  const showCreate = () => {
    setActionError(null)
    setEditing(null)
    form.setFieldsValue({ key: '', name: '', description: '' })
    setRoleOpen(true)
  }
  const showEdit = (role: Role) => {
    setActionError(null)
    setEditing(role)
    form.setFieldsValue({ key: role.key, name: role.name, description: role.description ?? '' })
    setRoleOpen(true)
  }
  const submit = async (values: RoleForm) => {
    setActionError(null)
    try {
      await save.mutateAsync(values)
      closeRole()
      void message.success('角色已保存')
    } catch (error: unknown) {
      setActionError(error)
    }
  }
  const showPermissions = (role: Role) => {
    setActionError(null)
    setPermissionRole(role)
    setSelectedPermissionIds((role.permissions ?? []).map((permission) => permission.id))
    setPermissionOpen(true)
  }
  const submitPermissions = async () => {
    setActionError(null)
    try {
      await savePermissions.mutateAsync()
      setPermissionOpen(false)
      setPermissionRole(null)
      setSelectedPermissionIds([])
      void message.success('角色权限已更新')
    } catch (error: unknown) {
      setActionError(error)
    }
  }
  const confirmRemove = (role: Role) => modal.confirm({
    title: '确认删除',
    content: `确认删除角色“${role.name}”吗？仍被成员使用的角色不能删除。`,
    okButtonProps: { danger: true },
    okText: '删除',
    onOk: async () => {
      setActionError(null)
      try {
        await remove.mutateAsync(role.id)
        void message.success('角色已删除')
      } catch (error: unknown) {
        setActionError(error)
        throw error
      }
    },
  })

  const queryError = roles.error ?? permissions.error
  return (
    <section className="management-panel">
      <header className="management-header">
        <div><h2>角色与权限</h2><p>权限清单由后端返回。页面不会补充未声明权限。</p></div>
        {systemAdmin ? <Button type="primary" onClick={showCreate}>新建角色</Button> : null}
      </header>
      {actionError ?? queryError ? <Alert className="management-alert" type="error" showIcon message={actionErrorMessage(actionError ?? queryError, actionError ? '执行角色操作' : '读取角色与权限')} /> : null}
      {!systemAdmin ? <Alert type="warning" showIcon message="只有系统管理员可以修改全局角色与权限" /> : (
        <Table<Role> rowKey="id" loading={roles.isLoading || permissions.isLoading} dataSource={roles.data ?? []} locale={{ emptyText: '暂无角色' }} pagination={false} scroll={{ x: 900 }} columns={[
          { title: '名称', dataIndex: 'name', width: 170 },
          { title: '标识', dataIndex: 'key', width: 170 },
          { title: '说明', dataIndex: 'description' },
          { title: '权限数', width: 90, render: (_, role) => role.permissions?.length ?? 0 },
          { title: '类型', width: 100, render: (_, role) => <Tag>{role.builtin ? '内置' : '自定义'}</Tag> },
          { title: '操作', width: 250, fixed: 'right', render: (_, role) => <div className="table-actions"><Button type="link" onClick={() => showEdit(role)}>编辑</Button><Button type="link" onClick={() => showPermissions(role)}>配置权限</Button>{!role.builtin ? <Button type="link" danger onClick={() => confirmRemove(role)}>删除</Button> : null}</div> },
        ]} />
      )}
      <Modal title={editing ? '编辑角色' : '新建角色'} open={roleOpen} width={520} confirmLoading={save.isPending} onCancel={closeRole} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <Form.Item label="角色标识" name="key" rules={[{ required: true, message: '请输入角色标识' }, { pattern: /^[a-z][a-z0-9_:.-]*$/, message: '角色标识格式不正确' }]}><Input disabled={Boolean(editing)} /></Form.Item>
          <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入角色名称' }]}><Input /></Form.Item>
          <Form.Item label="说明" name="description" rules={[{ max: 255, message: '说明不能超过 255 个字符' }]}><Input.TextArea rows={3} /></Form.Item>
        </Form>
      </Modal>
      <Modal
        title={permissionRole ? `配置“${permissionRole.name}”的权限` : '配置权限'}
        open={permissionOpen}
        width={660}
        confirmLoading={savePermissions.isPending}
        okButtonProps={{ disabled: !permissions.data?.length }}
        okText="保存权限"
        onCancel={() => { setPermissionOpen(false); setPermissionRole(null); setSelectedPermissionIds([]) }}
        onOk={submitPermissions}
        destroyOnHidden
      >
        {!permissions.data?.length ? <Alert type="warning" showIcon message="后端没有返回可分配权限" /> : (
          <Checkbox.Group aria-label="权限清单" className="permission-list" value={selectedPermissionIds} onChange={(values) => setSelectedPermissionIds(values)}>
            {permissions.data.map((permission) => (
              <label className="permission-item" key={permission.id}>
                <Checkbox value={permission.id}>{permission.name}（{permission.key}）</Checkbox>
                <span>{permission.description || '无说明'}</span>
              </label>
            ))}
          </Checkbox.Group>
        )}
      </Modal>
    </section>
  )
}
