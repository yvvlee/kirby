import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Form, Input, Modal, Table, Tooltip } from 'antd'
import { useState } from 'react'

import { createEnum, deleteEnum, updateEnum } from '@/api/enums'
import { getApiErrorMessage } from '@/api/errors'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import { requireEnumValues } from '../editor-validation'
import { type ConfigEnum, type ConfigEnumValue, useEnumsQuery } from '../queries'

type EnumForm = { key: string; name: string; description: string; values: ConfigEnumValue[] }
type Props = { projectId: Identifier; configId: Identifier; onChanged?: () => void }

const emptyValue = (): ConfigEnumValue => ({ label: '', value: '', description: '' })

export default function EnumsPanel({ projectId, configId, onChanged }: Props) {
  const { message, modal } = App.useApp()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const enums = useEnumsQuery(environment.currentId, projectId, configId)
  const canWrite = environment.hasPermission('enum:write')
  const [form] = Form.useForm<EnumForm>()
  const [editing, setEditing] = useState<ConfigEnum | null>(null)
  const [open, setOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const refresh = () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.enums(environment.currentId, projectId, configId) })
  const save = useMutation({
    mutationFn: (values: EnumForm) => {
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      requireEnumValues(values.values)
      const request = { key: values.key, name: values.name, description: values.description, values: values.values }
      return editing ? updateEnum(environment.currentId, { ...request, id: editing.id, version: editing.version }) : createEnum(environment.currentId, { ...request, config_id: configId })
    },
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: (id: Identifier) => { if (environment.currentId === null) throw new Error('当前没有可用环境'); return deleteEnum(environment.currentId, id) },
    onSuccess: refresh,
  })
  const close = () => { setOpen(false); setEditing(null); form.resetFields() }
  const showCreate = () => { setActionError(null); setEditing(null); form.setFieldsValue({ key: '', name: '', description: '', values: [emptyValue()] }); setOpen(true) }
  const showEdit = (item: ConfigEnum) => { setActionError(null); setEditing(item); form.setFieldsValue({ key: item.key, name: item.name, description: item.description ?? '', values: item.values.map((value) => ({ ...value })) }); setOpen(true) }
  const submit = async (values: EnumForm) => {
    setActionError(null)
    try { const wasEditing = Boolean(editing); await save.mutateAsync(values); close(); onChanged?.(); void message.success(wasEditing ? '枚举已更新' : '枚举已创建') } catch (error: unknown) { setActionError(error) }
  }
  const confirmRemove = (item: ConfigEnum) => modal.confirm({ title: '删除枚举', content: `确认删除枚举“${item.name}”吗？`, okText: '删除', okButtonProps: { danger: true }, onOk: async () => { try { await remove.mutateAsync(item.id); onChanged?.(); void message.success('枚举已删除') } catch (error: unknown) { setActionError(error); throw error } } })

  return (
    <section aria-label="枚举定义" className="resource-panel">
      <header className="resource-toolbar"><p>枚举值用于限制配置字段的可选范围。</p>{canWrite ? <Button type="primary" size="small" onClick={showCreate}>创建枚举</Button> : null}</header>
      {actionError ?? enums.error ? <Alert className="management-alert" type="error" showIcon message={getApiErrorMessage(actionError ?? enums.error, actionError ? '保存枚举失败' : '加载枚举失败')} /> : null}
      <Table<ConfigEnum> rowKey="id" loading={enums.isLoading} dataSource={enums.data ?? []} locale={{ emptyText: '当前配置还没有枚举' }} pagination={false} scroll={{ x: 850 }} columns={[
        { title: '枚举标识', dataIndex: 'key', width: 160 }, { title: '名称', dataIndex: 'name', width: 160 }, { title: '描述', dataIndex: 'description' }, { title: '枚举值数', width: 100, render: (_, item) => item.values.length }, { title: '更新时间', dataIndex: 'updatedAt', width: 190 },
        ...(canWrite ? [{ title: '操作', width: 150, fixed: 'right' as const, render: (_: unknown, item: ConfigEnum) => <div className="table-actions"><Button type="link" onClick={() => showEdit(item)}>编辑</Button><Button type="link" danger onClick={() => confirmRemove(item)}>删除</Button></div> }] : []),
      ]} />
      <Modal title={editing ? '编辑枚举' : '创建枚举'} open={open} width={900} confirmLoading={save.isPending} onCancel={close} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <div className="form-grid"><Form.Item label="枚举标识" name="key" rules={[{ required: true, message: '请输入枚举标识' }, { pattern: /^[A-Za-z][A-Za-z0-9]*$/, message: '枚举标识只能包含字母和数字，且以字母开头' }]}><Input disabled={Boolean(editing)} maxLength={64} /></Form.Item><Form.Item label="枚举名称" name="name" rules={[{ required: true, message: '请输入枚举名称' }]}><Input maxLength={64} /></Form.Item></div>
          <Form.Item label="枚举描述" name="description"><Input.TextArea rows={2} maxLength={255} showCount /></Form.Item>
          <Form.List name="values">{(fields, { add, remove: removeValue }) => <div className="editor-list"><div className="editor-list-header"><strong>枚举值</strong><Button icon={<PlusOutlined />} onClick={() => add(emptyValue())}>添加枚举值</Button></div>{fields.map((field, index) => <div className="editor-row enum-value-row" key={field.key}><Form.Item name={[field.name, 'label']} rules={[{ required: true, message: '请输入显示文本' }]}><Input aria-label={`第 ${index + 1} 个显示文本`} placeholder="显示文本" /></Form.Item><Form.Item name={[field.name, 'value']} rules={[{ required: true, message: '请输入枚举值' }]}><Input aria-label={`第 ${index + 1} 个枚举值`} placeholder="例如 ENABLED" /></Form.Item><Form.Item name={[field.name, 'description']}><Input aria-label={`第 ${index + 1} 个描述`} placeholder="描述" /></Form.Item><Tooltip title="删除枚举值"><Button aria-label="删除枚举值" danger type="text" icon={<DeleteOutlined />} onClick={() => removeValue(field.name)} /></Tooltip></div>)}</div>}</Form.List>
        </Form>
      </Modal>
    </section>
  )
}
