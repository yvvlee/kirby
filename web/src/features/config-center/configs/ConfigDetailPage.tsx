import { ArrowLeftOutlined } from '@ant-design/icons'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Descriptions, Form, Input, Modal, Switch, Tabs, Tooltip } from 'antd'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { updateConfig, updateConfigValue } from '@/api/configs'
import { getApiErrorMessage } from '@/api/errors'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import DataTypeSelector from '@/components/DataTypeSelector/DataTypeSelector'
import EnvironmentTag from '@/components/EnvironmentTag/EnvironmentTag'
import { createScopedFileUpload } from '@/components/FileUpload/scoped'
import MonacoEditor from '@/components/MonacoEditor/MonacoEditor'
import SchemaForm, { type SchemaFormHandle } from '@/components/SchemaForm/SchemaForm'
import { parseEditorType, stringifyEditorType, toApiType, toEditorType } from '@/domain/type-codec'
import EnumsPanel from '@/features/config-center/enums/EnumsPanel'
import ModelsPanel from '@/features/config-center/models/ModelsPanel'
import SnapshotsPanel from '@/features/config-center/snapshots/SnapshotsPanel'
import { useConfigDetailQuery, useEnumsQuery, useModelsQuery, useProjectQuery } from '../queries'
import { formatConfigJSON } from './detail-model'

type DefinitionForm = { description: string; type: string; isArray: boolean }

function routeId(value: string | undefined, name: string): number {
  if (!value || !/^[1-9]\d*$/.test(value)) throw new TypeError(`${name} 必须是正整数`)
  return Number(value)
}

export default function ConfigDetailPage() {
  const params = useParams()
  const projectId = routeId(params.projectId, 'projectId')
  const configId = routeId(params.configId, 'configId')
  const navigate = useNavigate()
  const { message } = App.useApp()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const project = useProjectQuery(environment.currentId, projectId)
  const detail = useConfigDetailQuery(environment.currentId, projectId, configId)
  const models = useModelsQuery(environment.currentId, projectId, configId)
  const enums = useEnumsQuery(environment.currentId, projectId, configId)
  const config = detail.data?.config
  const tree = detail.data?.tree
  const [activeTab, setActiveTab] = useState('content')
  const [previewValue, setPreviewValue] = useState('')
  const [definitionOpen, setDefinitionOpen] = useState(false)
  const [actionError, setActionError] = useState<unknown>(null)
  const [definitionForm] = Form.useForm<DefinitionForm>()
  const schemaForm = useRef<SchemaFormHandle>(null)
  const canWriteConfig = environment.hasPermission('config:write')
  const canReadSnapshots = environment.hasPermission('snapshot:read')
  const ScopedFileUpload = useMemo(() => environment.currentId === null ? undefined : createScopedFileUpload(environment.currentId, projectId), [environment.currentId, projectId])
  const refresh = () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.config(environment.currentId, projectId, configId) })
  const saveValue = useMutation({
    mutationFn: (value: unknown) => {
      if (environment.currentId === null || !config) throw new Error('配置尚未加载')
      return updateConfigValue(environment.currentId, { id: config.id, version: config.version, value: JSON.stringify(value) })
    },
    onSuccess: refresh,
  })
  const saveDefinition = useMutation({
    mutationFn: (values: DefinitionForm) => {
      if (environment.currentId === null || !config) throw new Error('配置尚未加载')
      return updateConfig(environment.currentId, { id: config.id, description: values.description, type: toApiType(parseEditorType(values.type)), is_array: Boolean(values.isArray), version: config.version })
    },
    onSuccess: refresh,
  })

  useEffect(() => {
    if (!config) { setPreviewValue(''); return }
    try { setPreviewValue(formatConfigJSON(config.value)) } catch (error: unknown) { setActionError(error) }
  }, [config])

  const preview = () => {
    if (!schemaForm.current) throw new Error('配置表单尚未加载')
    const value = schemaForm.current.getValue()
    setPreviewValue(JSON.stringify(value, null, 2))
    return value
  }
  const submitValue = async () => {
    setActionError(null)
    try { await saveValue.mutateAsync(preview()); void message.success('配置内容已保存') } catch (error: unknown) { setActionError(error) }
  }
  const openDefinition = () => {
    if (!config) throw new Error('配置尚未加载')
    definitionForm.setFieldsValue({ description: config.description ?? '', type: stringifyEditorType(config.type), isArray: config.isArray })
    setDefinitionOpen(true)
  }
  const submitDefinition = async (values: DefinitionForm) => {
    setActionError(null)
    try { await saveDefinition.mutateAsync(values); setDefinitionOpen(false); definitionForm.resetFields(); void message.success('配置定义已更新') } catch (error: unknown) { setActionError(error) }
  }
  const editorType = config ? toEditorType(config.type) : null
  const typeLabel = editorType?.baseType ?? editorType?.structureKey ?? editorType?.enumKey ?? '未设置'
  const queryError = project.error ?? detail.error ?? models.error ?? enums.error

  return (
    <section className="config-detail" aria-labelledby="config-detail-title">
      <header className="detail-header"><Tooltip title="返回配置列表"><Button aria-label="返回配置列表" type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate(`/projects/${projectId}/configs`)} /></Tooltip><div className="title-row"><h1 id="config-detail-title">{config?.key ?? '配置详情'}</h1><EnvironmentTag environment={environment.current} /></div></header>
      {actionError ?? queryError ? <Alert type="error" showIcon message={getApiErrorMessage(actionError ?? queryError, actionError ? '配置操作失败' : '加载配置详情失败')} /> : null}
      {config ? <section className="config-summary"><header className="catalog-header"><div><h2>{config.description || config.key}</h2><p>{project.data?.name ?? ''} · {config.key}</p></div>{canWriteConfig ? <Button type="primary" onClick={openDefinition}>修改配置定义</Button> : null}</header><Descriptions bordered size="small" column={{ xs: 1, sm: 3 }} items={[{ key: 'type', label: '数据类型', children: typeLabel }, { key: 'array', label: '数组', children: config.isArray ? '是' : '否' }, { key: 'version', label: '版本', children: config.version }]} /></section> : null}
      {config ? <section className="config-workspace"><Tabs activeKey={activeTab} onChange={setActiveTab} items={[
        { key: 'content', label: '配置内容', children: <><div className="config-editor-grid"><div className="form-pane">{tree && ScopedFileUpload ? <SchemaForm key={`${environment.currentId}:${projectId}:${configId}`} ref={schemaForm} config={tree} value={config.value} disabled={!canWriteConfig} models={models.data ?? []} enums={enums.data ?? []} fileUploadComponent={ScopedFileUpload} /> : <Alert type="warning" message="配置结构不可用，请先修改配置定义。" />}</div><div className="json-pane"><MonacoEditor value={previewValue} disabled /></div></div>{canWriteConfig && tree ? <div className="content-actions"><Button onClick={() => { try { preview() } catch (error: unknown) { setActionError(error) } }}>刷新 JSON 预览</Button><Button type="primary" loading={saveValue.isPending} onClick={() => void submitValue()}>保存配置内容</Button></div> : null}</> },
        { key: 'models', label: '模型定义', children: activeTab === 'models' ? <ModelsPanel projectId={projectId} configId={configId} enums={enums.data ?? []} onChanged={() => void refresh()} /> : null },
        { key: 'enums', label: '枚举定义', children: activeTab === 'enums' ? <EnumsPanel projectId={projectId} configId={configId} onChanged={() => void refresh()} /> : null },
        ...(canReadSnapshots ? [{ key: 'snapshots', label: '快照', children: activeTab === 'snapshots' ? <SnapshotsPanel projectId={projectId} configId={configId} onChanged={() => void refresh()} /> : null }] : []),
      ]} /></section> : null}
      <Modal title="修改配置定义" open={definitionOpen} width={560} confirmLoading={saveDefinition.isPending} onCancel={() => { setDefinitionOpen(false); definitionForm.resetFields() }} onOk={() => definitionForm.submit()} afterClose={() => definitionForm.resetFields()} destroyOnHidden>
        <Form form={definitionForm} layout="vertical" onFinish={submitDefinition} preserve={false}>
          <Form.Item label="配置标识"><Input value={config?.key} disabled /></Form.Item><Form.Item label="配置描述" name="description"><Input maxLength={255} showCount /></Form.Item><Form.Item label="数据类型" name="type" rules={[{ required: true, message: '请选择数据类型' }]}><DataTypeSelector models={models.data ?? []} enums={enums.data ?? []} /></Form.Item><Form.Item label="数组" name="isArray" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </section>
  )
}
