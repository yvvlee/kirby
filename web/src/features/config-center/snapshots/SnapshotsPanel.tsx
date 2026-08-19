import { DownloadOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Alert, App, Button, Form, Input, Modal, Pagination, Select, Space, Table, Tag } from 'antd'
import { useRef, useState } from 'react'

import { getMyPermissions } from '@/api/environments'
import { listConfigs } from '@/api/configs'
import { listProjects } from '@/api/projects'
import { createImportIdempotencyKey, exportSnapshot, importSnapshot, publishSnapshot, unpublishSnapshot, type ImportConflictStrategy } from '@/api/snapshot-imports'
import { createSnapshot, deleteSnapshot, getCurrentSnapshot, getSnapshot, listSnapshots, loadSnapshot, previewCreatingSnapshot } from '@/api/snapshots'
import type { Identifier } from '@/api/types'
import { queryKeys } from '@/app/query-keys'
import { useEnvironment } from '@/auth/environment-state'
import DiffEditor from '@/components/DiffEditor/DiffEditor'
import { normalizeSnapshotList, parseSnapshotContent, SNAPSHOT_TAG_OPTIONS, snapshotActionError, snapshotStatusLabel, type SnapshotStatus } from '@/domain/snapshots'
import { normalizeConfigList, type ConfigSummary, type Project } from '../queries'
import { requireList } from '@/features/system/errors'
import { nextImportRequestIdentity } from './import-request'

type SnapshotRow = Record<string, unknown> & {
  id: Identifier
  description: string
  version: number
  createdBy?: string
  createdAt?: string
  tags: string[]
  status: SnapshotStatus
}
type CreateForm = { description: string; tags: string[] }
type ImportForm = {
  targetEnvironmentId: Identifier
  targetProjectId: Identifier
  targetConfigId?: Identifier
  conflictStrategy: ImportConflictStrategy
  description: string
  tags: string[]
}
type Props = { projectId: Identifier; configId: Identifier; onChanged?: () => void }

function requireSnapshot(reply: unknown, action: string): Record<string, unknown> {
  if (typeof reply !== 'object' || reply === null || !('snapshot' in reply) || typeof reply.snapshot !== 'object' || reply.snapshot === null) {
    throw new TypeError(`${action}响应缺少 snapshot`)
  }
  return reply.snapshot as Record<string, unknown>
}

function contentOf(snapshot: Record<string, unknown>): unknown {
  if (typeof snapshot.content !== 'string') throw new TypeError('快照内容必须是非空 JSON 字符串')
  return parseSnapshotContent(snapshot.content)
}

export default function SnapshotsPanel({ projectId, configId, onChanged }: Props) {
  const { message, modal } = App.useApp()
  const environment = useEnvironment()
  const queryClient = useQueryClient()
  const [page, setPage] = useState({ page: 1, limit: 10 })
  const [createForm] = Form.useForm<CreateForm>()
  const [importForm] = Form.useForm<ImportForm>()
  const [createOpen, setCreateOpen] = useState(false)
  const [createDiff, setCreateDiff] = useState<{ current: unknown; preview: unknown } | null>(null)
  const [compareOpen, setCompareOpen] = useState(false)
  const [compareDiff, setCompareDiff] = useState<{ current: unknown; snapshot: unknown } | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [sourceSnapshot, setSourceSnapshot] = useState<SnapshotRow | null>(null)
  const [targetAllowed, setTargetAllowed] = useState(false)
  const [targetLoading, setTargetLoading] = useState(false)
  const [targetProjects, setTargetProjects] = useState<Project[]>([])
  const [targetConfigs, setTargetConfigs] = useState<ConfigSummary[]>([])
  const [actionError, setActionError] = useState<unknown>(null)
  const [importError, setImportError] = useState('')
  const importRequest = useRef({ signature: '', key: '' })
  const canWrite = environment.hasPermission('snapshot:write')
  const canPublish = environment.hasPermission('snapshot:publish')
  const canExport = environment.hasPermission('snapshot:export')
  const snapshots = useQuery({
    queryKey: environment.currentId === null ? ['environment', 'none', 'snapshots'] : queryKeys.snapshots(environment.currentId, projectId, configId, { page }),
    queryFn: async () => {
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      const normalized = normalizeSnapshotList(await listSnapshots(environment.currentId, { project_id: projectId, config_id: configId, page }))
      return { list: normalized.list as SnapshotRow[], page: { page: Number(normalized.page.page), limit: Number(normalized.page.limit), total: Number(normalized.page.total) } }
    },
    enabled: environment.currentId !== null,
  })
  const refresh = () => environment.currentId === null ? Promise.resolve() : queryClient.invalidateQueries({ queryKey: queryKeys.snapshots(environment.currentId, projectId, configId) })
  const create = useMutation({ mutationFn: (values: CreateForm) => { if (environment.currentId === null) throw new Error('当前没有可用环境'); return createSnapshot(environment.currentId, { config_id: configId, project_id: projectId, description: values.description, tags: values.tags }) }, onSuccess: refresh })
  const transfer = useMutation({
    mutationFn: async ({ operation, row }: { operation: 'restore' | 'publish' | 'unpublish' | 'delete'; row: SnapshotRow }) => {
      if (environment.currentId === null) throw new Error('当前没有可用环境')
      if (operation === 'restore') return loadSnapshot(environment.currentId, row.id, configId)
      if (operation === 'publish') return publishSnapshot(environment.currentId, row.id, row.version)
      if (operation === 'unpublish') return unpublishSnapshot(environment.currentId, row.id, row.version)
      return deleteSnapshot(environment.currentId, row.id)
    },
    onSuccess: refresh,
  })
  const importing = useMutation({
    mutationFn: ({ targetEnvironmentId, request }: { targetEnvironmentId: Identifier; request: Parameters<typeof importSnapshot>[1] }) => importSnapshot(targetEnvironmentId, request),
  })

  const openCreate = async () => {
    if (!canWrite) throw new Error('当前用户没有创建快照权限')
    if (environment.currentId === null) throw new Error('当前没有可用环境')
    setActionError(null); setCreateDiff(null); createForm.setFieldsValue({ description: '', tags: [] }); setCreateOpen(true)
    try {
      const [currentReply, previewReply] = await Promise.all([getCurrentSnapshot(environment.currentId, configId), previewCreatingSnapshot(environment.currentId, configId)])
      const current = requireSnapshot(currentReply, '当前快照')
      if (typeof previewReply.content !== 'string') throw new TypeError('快照预览响应缺少 content')
      setCreateDiff({ current: contentOf(current), preview: parseSnapshotContent(previewReply.content) })
    } catch (error: unknown) { setActionError(error) }
  }
  const submitCreate = async (values: CreateForm) => {
    setActionError(null)
    try { await create.mutateAsync(values); setCreateOpen(false); setCreateDiff(null); createForm.resetFields(); void message.success('快照已创建') } catch (error: unknown) { setActionError(error) }
  }
  const openCompare = async (row: SnapshotRow) => {
    if (environment.currentId === null) throw new Error('当前没有可用环境')
    setActionError(null); setCompareDiff(null); setCompareOpen(true)
    try { const [currentReply, snapshotReply] = await Promise.all([getCurrentSnapshot(environment.currentId, configId), getSnapshot(environment.currentId, row.id)]); setCompareDiff({ current: contentOf(requireSnapshot(currentReply, '当前快照')), snapshot: contentOf(requireSnapshot(snapshotReply, '快照详情')) }) } catch (error: unknown) { setActionError(error) }
  }
  const execute = async (operation: 'restore' | 'publish' | 'unpublish' | 'delete', row: SnapshotRow) => {
    setActionError(null)
    try { await transfer.mutateAsync({ operation, row }); if (operation === 'restore') onChanged?.(); void message.success({ restore: '快照已还原', publish: '快照已发布', unpublish: '快照已下线', delete: '快照已删除' }[operation]) } catch (error: unknown) { setActionError(error); throw error }
  }
  const confirmRestore = (row: SnapshotRow) => modal.confirm({ title: '还原快照', content: '还原会覆盖尚未保存为快照的配置，是否继续？', okText: '还原', onOk: () => execute('restore', row) })
  const confirmDelete = (row: SnapshotRow) => modal.confirm({ title: '删除快照', content: `确认删除快照“${row.description}”吗？`, okText: '删除', okButtonProps: { danger: true }, onOk: () => execute('delete', row) })
  const download = async (row: SnapshotRow) => {
    if (!canExport) throw new Error('当前用户没有导出快照权限')
    if (environment.currentId === null) throw new Error('当前没有可用环境')
    setActionError(null)
    try {
      const reply = await exportSnapshot(environment.currentId, row.id)
      const url = URL.createObjectURL(new Blob([JSON.stringify(reply, null, 2)], { type: 'application/json' }))
      try { const link = document.createElement('a'); link.href = url; link.download = `kirby-snapshot-${row.id}.json`; link.click() } finally { URL.revokeObjectURL(url) }
    } catch (error: unknown) { setActionError(error) }
  }
  const openImport = (row: SnapshotRow) => {
    if (!canExport) throw new Error('当前用户没有导出源快照权限')
    const key = createImportIdempotencyKey()
    importRequest.current = { signature: '', key }
    setSourceSnapshot(row); setTargetAllowed(false); setTargetProjects([]); setTargetConfigs([]); setImportError('')
    importForm.setFieldsValue({ conflictStrategy: 'FAIL', description: row.description, tags: [...row.tags] })
    setImportOpen(true)
  }
  const loadTargetEnvironment = async (targetEnvironmentId: Identifier) => {
    setTargetLoading(true); setImportError(''); setTargetAllowed(false); setTargetProjects([]); setTargetConfigs([]); importForm.resetFields(['targetProjectId', 'targetConfigId'])
    try {
      const permissions = await getMyPermissions(targetEnvironmentId)
      if (!Array.isArray(permissions.permissions)) throw new TypeError('目标环境权限响应缺少 permissions')
      const declared = new Set(permissions.permissions)
      if (!declared.has('snapshot:import') || !declared.has('config:write')) { setImportError('当前用户没有目标环境的快照导入或配置写入权限。'); return }
      setTargetAllowed(true)
      setTargetProjects(requireList<Project>(await listProjects(targetEnvironmentId, {}), 'target project list'))
    } catch (error: unknown) { setImportError(snapshotActionError(error, '读取目标环境权限和项目', '当前导入表单已保留。')) } finally { setTargetLoading(false) }
  }
  const loadTargetConfigs = async (targetProjectId: Identifier) => {
    setTargetConfigs([]); setImportError(''); importForm.setFieldValue('targetConfigId', undefined)
    const targetEnvironmentId = importForm.getFieldValue('targetEnvironmentId')
    try { setTargetConfigs(normalizeConfigList(await listConfigs(targetEnvironmentId, { project_id: targetProjectId }))) } catch (error: unknown) { setImportError(snapshotActionError(error, '读取目标配置', '当前导入表单已保留。')) }
  }
  const submitImport = async (values: ImportForm) => {
    if (!targetAllowed) throw new Error('目标环境没有快照导入权限')
    if (!sourceSnapshot || environment.currentId === null) throw new Error('源快照尚未选择')
    if (values.conflictStrategy === 'REPLACE' && !values.targetConfigId) { setImportError('替换策略必须选择目标配置。'); return }
    const base = { source_environment_id: environment.currentId, source_snapshot_id: sourceSnapshot.id, target_project_id: values.targetProjectId, description: values.description, tags: values.tags, conflict_strategy: values.conflictStrategy, ...(values.conflictStrategy === 'REPLACE' ? { target_config_id: values.targetConfigId } : {}) }
    const signature = JSON.stringify({ ...base, target_environment_id: values.targetEnvironmentId })
    importRequest.current = nextImportRequestIdentity(importRequest.current, signature, createImportIdempotencyKey)
    setImportError('')
    try {
      const reply = await importing.mutateAsync({ targetEnvironmentId: values.targetEnvironmentId, request: { ...base, idempotency_key: importRequest.current.key } })
      if (!reply.snapshot) throw new TypeError('快照导入响应缺少 snapshot')
      void message.success(reply.replayed ? '已返回上次导入结果' : '快照已导入')
      setImportOpen(false); setSourceSnapshot(null); importForm.resetFields()
    } catch (error: unknown) { setImportError(snapshotActionError(error, '从源环境导出或向目标环境导入', '当前导入表单已保留，可重试同一请求。')) }
  }

  const error = actionError ?? snapshots.error
  const conflictStrategy = Form.useWatch('conflictStrategy', importForm)
  return (
    <section aria-label="快照管理" className="resource-panel">
      {error ? <Alert className="management-alert" type="error" showIcon message={snapshotActionError(error, actionError ? '执行快照操作' : '读取快照')} /> : null}
      <header className="resource-toolbar"><p>快照用于比较、还原、发布和跨环境复用配置。</p><Space><Button size="small" icon={<ReloadOutlined />} loading={snapshots.isFetching} onClick={() => void snapshots.refetch()}>刷新</Button>{canWrite ? <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => void openCreate()}>创建快照</Button> : null}</Space></header>
      <Table<SnapshotRow> rowKey="id" loading={snapshots.isLoading} dataSource={snapshots.data?.list ?? []} locale={{ emptyText: '当前配置还没有快照' }} pagination={false} scroll={{ x: 1180 }} columns={[
        { title: 'ID', dataIndex: 'id', width: 80 }, { title: '描述', dataIndex: 'description', width: 220 }, { title: '标签', width: 180, render: (_, row) => row.tags.map((tag) => <Tag key={tag}>{tag}</Tag>) }, { title: '状态', width: 110, render: (_, row) => <Tag color={row.status === 'RELEASED' ? 'green' : 'default'}>{snapshotStatusLabel(row.status)}</Tag> }, { title: '创建人', dataIndex: 'createdBy', width: 130 }, { title: '创建时间', dataIndex: 'createdAt', width: 190 },
        { title: '操作', width: 430, fixed: 'right', render: (_, row) => <div className="table-actions"><Button type="link" onClick={() => void openCompare(row)}>比较</Button>{canWrite ? <Button type="link" onClick={() => confirmRestore(row)}>还原</Button> : null}{canPublish && row.status === 'UNRELEASED' ? <Button type="link" onClick={() => void execute('publish', row).catch(() => undefined)}>发布</Button> : null}{canPublish && row.status === 'RELEASED' ? <Button type="link" onClick={() => void execute('unpublish', row).catch(() => undefined)}>下线</Button> : null}{canExport ? <Button type="link" icon={<DownloadOutlined />} onClick={() => void download(row)}>导出</Button> : null}{canExport ? <Button type="link" onClick={() => openImport(row)}>导入到环境</Button> : null}{canWrite && row.status === 'UNRELEASED' ? <Button type="link" danger onClick={() => confirmDelete(row)}>删除</Button> : null}</div> },
      ]} />
      <Pagination className="resource-pagination" current={snapshots.data?.page.page || page.page} pageSize={snapshots.data?.page.limit || page.limit} total={snapshots.data?.page.total || 0} showSizeChanger pageSizeOptions={[10, 20, 50, 100]} onChange={(nextPage, limit) => setPage({ page: limit === page.limit ? nextPage : 1, limit })} />
      <Modal title="创建快照" open={createOpen} width={1100} confirmLoading={create.isPending} okText="创建" onCancel={() => { setCreateOpen(false); setCreateDiff(null); createForm.resetFields() }} onOk={() => createForm.submit()} destroyOnHidden>
        <Form form={createForm} layout="vertical" onFinish={submitCreate} preserve={false}><Form.Item label="快照标签" name="tags" rules={[{ required: true, type: 'array', min: 1, message: '请选择快照标签' }]}><Select mode="multiple" options={[...SNAPSHOT_TAG_OPTIONS]} /></Form.Item><Form.Item label="快照描述" name="description" rules={[{ required: true, message: '请输入快照描述' }, { min: 2, max: 255, message: '快照描述长度为 2 到 255 个字符' }]}><Input.TextArea rows={3} maxLength={255} showCount /></Form.Item><p className="secondary-text">左侧是当前配置，右侧是待创建快照。</p>{createDiff ? <DiffEditor leftValue={createDiff.current} rightValue={createDiff.preview} /> : <p>正在读取快照预览…</p>}</Form>
      </Modal>
      <Modal title="比较快照" open={compareOpen} width={1100} footer={null} onCancel={() => { setCompareOpen(false); setCompareDiff(null) }} destroyOnHidden><p className="secondary-text">左侧是当前配置，右侧是所选快照。</p>{compareDiff ? <DiffEditor leftValue={compareDiff.current} rightValue={compareDiff.snapshot} /> : <p>正在读取比较内容…</p>}</Modal>
      <Modal title="导入快照到目标环境" open={importOpen} width={680} confirmLoading={importing.isPending} okText="导入" okButtonProps={{ disabled: !targetAllowed }} onCancel={() => { setImportOpen(false); setSourceSnapshot(null); setImportError(''); importForm.resetFields() }} onOk={() => importForm.submit()} destroyOnHidden>
        {importError ? <Alert className="management-alert" type="error" showIcon message={importError} /> : null}
        <Form form={importForm} layout="vertical" onFinish={submitImport} preserve={false}><Form.Item label="源快照"><Input value={`环境 ${environment.currentId} / 快照 ${sourceSnapshot?.id ?? ''}`} disabled /></Form.Item><Form.Item label="目标环境" name="targetEnvironmentId" rules={[{ required: true, message: '请选择目标环境' }]}><Select loading={targetLoading} options={environment.available.map((item) => ({ label: item.name, value: item.id, disabled: !item.enabled }))} onChange={(value) => void loadTargetEnvironment(value)} /></Form.Item><Form.Item label="目标项目" name="targetProjectId" rules={[{ required: true, message: '请选择目标项目' }]}><Select disabled={!targetAllowed} options={targetProjects.map((item) => ({ label: item.name, value: item.id }))} onChange={(value) => void loadTargetConfigs(value)} /></Form.Item><Form.Item label="冲突策略" name="conflictStrategy" rules={[{ required: true, message: '请选择冲突策略' }]}><Select options={[{ label: '冲突时报错并创建新配置', value: 'FAIL' }, { label: '替换指定配置', value: 'REPLACE' }]} /></Form.Item>{conflictStrategy === 'REPLACE' ? <Form.Item label="目标配置" name="targetConfigId" rules={[{ required: true, message: '请选择目标配置' }]}><Select options={targetConfigs.map((item) => ({ label: item.description || item.key, value: item.id }))} /></Form.Item> : null}<Form.Item label="快照描述" name="description" rules={[{ required: true, message: '请输入快照描述' }, { min: 2, max: 255, message: '快照描述长度为 2 到 255 个字符' }]}><Input maxLength={255} /></Form.Item><Form.Item label="快照标签" name="tags"><Select mode="multiple" options={[...SNAPSHOT_TAG_OPTIONS]} /></Form.Item></Form>
      </Modal>
    </section>
  )
}
