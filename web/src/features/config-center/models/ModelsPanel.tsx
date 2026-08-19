import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Checkbox, Form, Input, Modal, Space, Table, Tooltip } from 'antd'
import { useMemo, useState } from 'react'

import { createModel, deleteModel, updateModel } from '@/api/models'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import DataTypeSelector from '@/components/DataTypeSelector/DataTypeSelector'
import { getApiErrorMessage } from '@/api/errors'
import { parseEditorType, toApiType } from '@/domain/type-codec'
import { requireModelFields, type EditorModelField } from '../editor-validation'
import { type ConfigEnum, type ConfigModel, useModelsQuery } from '../queries'

type ModelForm = { key: string; name: string; description: string; fields: EditorModelField[] }
type Props = { projectId: Identifier; configId: Identifier; enums?: ConfigEnum[]; onChanged?: () => void }

export default function ModelsPanel({ projectId, configId, enums = [], onChanged }: Props) {
  const { message, modal } = App.useApp()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const models = useModelsQuery(environment.currentId, projectId, configId)
  const canWrite = environment.hasPermission('structure:write')
  const [form] = Form.useForm<ModelForm>()
  const [editing, setEditing] = useState<ConfigModel | null>(null)
  const [open, setOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const limitedModels = useMemo(() => (models.data ?? []).filter((item) => item.id !== editing?.id), [editing?.id, models.data])
  const refresh = () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.models(environment.currentId, projectId, configId) })
  const save = useMutation({
    mutationFn: (values: ModelForm) => {
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      if (!editing) return createModel(environment.currentId, { config_id: configId, key: values.key, name: values.name, description: values.description })
      requireModelFields(values.fields)
      return updateModel(environment.currentId, {
        id: editing.id,
        key: values.key,
        name: values.name,
        description: values.description,
        version: editing.version,
        fields: values.fields.map((field) => ({ key: field.key, name: field.name, description: field.description || '', is_array: Boolean(field.isArray), type: toApiType(parseEditorType(field.type)) })),
      })
    },
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: (id: Identifier) => {
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      return deleteModel(environment.currentId, id)
    },
    onSuccess: refresh,
  })

  const close = () => { setOpen(false); setEditing(null); form.resetFields() }
  const showCreate = () => { setActionError(null); setEditing(null); form.setFieldsValue({ key: '', name: '', description: '', fields: [] }); setOpen(true) }
  const showEdit = (model: ConfigModel) => {
    setActionError(null)
    setEditing(model)
    form.setFieldsValue({ key: model.key, name: model.name, description: model.description ?? '', fields: model.fields.map((field) => ({ key: field.key, name: field.name ?? field.key, description: field.description ?? '', isArray: Boolean(field.isArray), type: JSON.stringify(field.type) })) })
    setOpen(true)
  }
  const submit = async (values: ModelForm) => {
    setActionError(null)
    try {
      const wasEditing = Boolean(editing)
      await save.mutateAsync(values)
      close()
      onChanged?.()
      void message.success(wasEditing ? '模型已更新' : '模型已创建')
    } catch (error: unknown) { setActionError(error) }
  }
  const confirmRemove = (model: ConfigModel) => modal.confirm({
    title: '删除模型', content: `确认删除模型“${model.name}”吗？`, okText: '删除', okButtonProps: { danger: true },
    onOk: async () => { try { await remove.mutateAsync(model.id); onChanged?.(); void message.success('模型已删除') } catch (error: unknown) { setActionError(error); throw error } },
  })

  return (
    <section aria-label="模型定义" className="resource-panel">
      <header className="resource-toolbar"><p>模型可以被当前配置的字段引用。</p>{canWrite ? <Button type="primary" size="small" onClick={showCreate}>创建模型</Button> : null}</header>
      {actionError ?? models.error ? <Alert className="management-alert" type="error" showIcon message={getApiErrorMessage(actionError ?? models.error, actionError ? '保存模型失败' : '加载模型失败')} /> : null}
      <Table<ConfigModel> rowKey="id" loading={models.isLoading} dataSource={models.data ?? []} locale={{ emptyText: '当前配置还没有模型' }} pagination={false} scroll={{ x: 850 }} columns={[
        { title: '模型标识', dataIndex: 'key', width: 160 }, { title: '名称', dataIndex: 'name', width: 160 }, { title: '描述', dataIndex: 'description' },
        { title: '字段数', width: 90, render: (_, model) => model.fields.length }, { title: '更新时间', dataIndex: 'updatedAt', width: 190 },
        ...(canWrite ? [{ title: '操作', width: 150, fixed: 'right' as const, render: (_: unknown, model: ConfigModel) => <div className="table-actions"><Button type="link" onClick={() => showEdit(model)}>编辑</Button><Button type="link" danger onClick={() => confirmRemove(model)}>删除</Button></div> }] : []),
      ]} />
      <Modal title={editing ? '编辑模型' : '创建模型'} open={open} width={1100} confirmLoading={save.isPending} onCancel={close} onOk={() => form.submit()} afterClose={() => form.resetFields()} destroyOnHidden>
        <Form form={form} layout="vertical" onFinish={submit} preserve={false}>
          <div className="form-grid"><Form.Item label="模型标识" name="key" rules={[{ required: true, message: '请输入模型标识' }, { pattern: /^[A-Za-z][A-Za-z0-9]*$/, message: '模型标识只能包含字母和数字，且以字母开头' }]}><Input disabled={Boolean(editing)} maxLength={64} /></Form.Item><Form.Item label="模型名称" name="name" rules={[{ required: true, message: '请输入模型名称' }]}><Input maxLength={64} /></Form.Item></div>
          <Form.Item label="模型描述" name="description"><Input.TextArea rows={2} maxLength={255} showCount /></Form.Item>
          {editing ? <Form.List name="fields">{(fields, { add, remove: removeField, move }) => <div className="editor-list"><div className="editor-list-header"><strong>字段</strong><Button icon={<PlusOutlined />} onClick={() => add({ key: '', name: '', description: '', isArray: false, type: JSON.stringify({ baseType: 'String' }) })}>添加字段</Button></div>{fields.map((field, index) => <div className="editor-row model-field-row" key={field.key}><Space.Compact className="move-buttons"><Tooltip title="上移字段"><Button aria-label="上移字段" icon={<ArrowUpOutlined />} disabled={index === 0} onClick={() => move(index, index - 1)} /></Tooltip><Tooltip title="下移字段"><Button aria-label="下移字段" icon={<ArrowDownOutlined />} disabled={index === fields.length - 1} onClick={() => move(index, index + 1)} /></Tooltip></Space.Compact><Form.Item name={[field.name, 'key']} rules={[{ required: true, message: '请输入字段标识' }]}><Input placeholder="字段标识" /></Form.Item><Form.Item name={[field.name, 'name']} rules={[{ required: true, message: '请输入字段名称' }]}><Input placeholder="字段名称" /></Form.Item><Form.Item name={[field.name, 'type']} rules={[{ required: true, message: '请选择字段类型' }]}><DataTypeSelector models={models.data ?? []} limitedModels={limitedModels} enums={enums} limit /></Form.Item><Form.Item name={[field.name, 'description']}><Input placeholder="描述" /></Form.Item><Form.Item name={[field.name, 'isArray']} valuePropName="checked"><Checkbox>数组</Checkbox></Form.Item><Tooltip title="删除字段"><Button aria-label="删除字段" danger type="text" icon={<DeleteOutlined />} onClick={() => removeField(field.name)} /></Tooltip></div>)}</div>}</Form.List> : null}
        </Form>
      </Modal>
    </section>
  )
}
