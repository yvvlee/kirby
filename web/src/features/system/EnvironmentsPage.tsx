import { useMutation } from '@tanstack/react-query'
import { Alert, App, Button, Form, Input, Modal, Switch, Table, Tag } from 'antd'
import { useState } from 'react'

import { createEnvironment, updateEnvironment } from '@/api/environments'
import type { Environment } from '@/api/types'
import { useAuth } from '@/auth/auth-state'
import { useEnvironment } from '@/auth/environment-state'
import { actionErrorMessage } from './errors'

type EnvironmentForm = {
  key: string
  name: string
  description: string
  enabled: boolean
}

export default function EnvironmentsPage() {
  const { message } = App.useApp()
  const { systemAdmin } = useAuth()
  const environmentState = useEnvironment()
  const [form] = Form.useForm<EnvironmentForm>()
  const [editing, setEditing] = useState<Environment | null>(null)
  const [open, setOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const save = useMutation({
    mutationFn: async (values: EnvironmentForm) => editing
      ? updateEnvironment(editing.id, { ...values, version: editing.version })
      : createEnvironment({ key: values.key, name: values.name, description: values.description }),
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
    form.setFieldsValue({ key: '', name: '', description: '', enabled: true })
    setOpen(true)
  }
  const showEdit = (item: Environment) => {
    setActionError(null)
    setEditing(item)
    form.setFieldsValue({
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
      void message.success('环境已保存')
    } catch (error: unknown) {
      setActionError(error)
    }
  }

  const error = actionError ?? environmentState.error
  return (
    <section className="management-panel">
      <header className="management-header">
        <div><h2>环境</h2><p>环境是项目、成员角色和配置数据的隔离边界。</p></div>
        {systemAdmin ? <Button type="primary" onClick={showCreate}>新建环境</Button> : null}
      </header>
      {error ? <Alert className="management-alert" type="error" showIcon message={actionErrorMessage(error, '读取环境')} /> : null}
      {!systemAdmin ? <Alert type="warning" showIcon message="只有系统管理员可以修改环境" /> : (
        <Table<Environment>
          rowKey="id"
          loading={!environmentState.initialized || environmentState.switching}
          dataSource={environmentState.available}
          locale={{ emptyText: '暂无环境' }}
          pagination={false}
          scroll={{ x: 760 }}
          columns={[
            { title: '名称', dataIndex: 'name', width: 180 },
            { title: '标识', dataIndex: 'key', width: 160 },
            { title: '说明', dataIndex: 'description' },
            { title: '状态', width: 100, render: (_, item) => <Tag color={item.enabled ? 'green' : 'default'}>{item.enabled ? '启用' : '停用'}</Tag> },
            { title: '操作', width: 90, fixed: 'right', render: (_, item) => <Button type="link" onClick={() => showEdit(item)}>编辑</Button> },
          ]}
        />
      )}
      <Modal
        title={editing ? '编辑环境' : '新建环境'}
        open={open}
        width={520}
        confirmLoading={save.isPending}
        okText="保存"
        onCancel={close}
        onOk={() => form.submit()}
        afterClose={() => form.resetFields()}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <Form.Item label="环境标识" name="key" rules={[
            { required: true, message: '请输入环境标识' },
            { pattern: /^[a-z][a-z0-9-]*$/, message: '只能使用小写字母、数字和连字符，且必须以字母开头' },
          ]}>
            <Input disabled={Boolean(editing)} placeholder="例如 production" />
          </Form.Item>
          <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入环境名称' }]}><Input /></Form.Item>
          <Form.Item label="说明" name="description" rules={[{ max: 255, message: '说明不能超过 255 个字符' }]}><Input.TextArea rows={3} /></Form.Item>
          {editing ? <Form.Item label="状态" name="enabled" valuePropName="checked"><Switch checkedChildren="启用" unCheckedChildren="停用" /></Form.Item> : null}
        </Form>
      </Modal>
    </section>
  )
}
