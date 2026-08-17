// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import Vue from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const snapshotApi = vi.hoisted(() => ({
  createSnapshot: vi.fn(),
  deleteSnapshot: vi.fn(),
  getCurrentSnapshot: vi.fn(),
  getSnapshot: vi.fn(),
  loadSnapshot: vi.fn(),
  previewCreatingSnapshot: vi.fn(),
}))
const transferApi = vi.hoisted(() => ({
  createImportIdempotencyKey: vi.fn(() => 'kirby-import-fixed-12345'),
  exportSnapshot: vi.fn(),
  importSnapshot: vi.fn(),
  publishSnapshot: vi.fn(),
  unpublishSnapshot: vi.fn(),
}))
const environmentApi = vi.hoisted(() => ({
  getMyPermissions: vi.fn(),
}))

vi.mock('@/api/snapshots', () => snapshotApi)
vi.mock('@/api/snapshot-imports', () => transferApi)
vi.mock('@/api/environments', () => environmentApi)
vi.mock('@/components/DiffEditor', () => ({
  default: { name: 'DiffEditor', render: (createElement) => createElement('div') },
}))

import SnapshotsPanel from './SnapshotsPanel.vue'
import {
  normalizeSnapshotList,
  snapshotStatusLabel,
} from './model'

Vue.config.ignoredElements = [/^el-/]
Vue.config.productionTip = false
Vue.directive('loading', {})

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0))
}

function listReply() {
  return {
    list: [
      {
        id: 31,
        description: 'release',
        status: 'UNRELEASED',
        tags: ['RELEASE'],
        version: 2,
      },
    ],
    page: { page: 1, limit: 10, total: 1 },
  }
}

function mountPanel(permissions = []) {
  const dispatch = vi.fn().mockResolvedValue(listReply())
  const wrapper = shallowMount(SnapshotsPanel, {
    propsData: { projectId: 7, configId: 9 },
    mocks: {
      $store: {
        state: {
          environment: {
            currentId: 11,
            available: [{ id: 22, name: 'West', enabled: true }],
          },
        },
        getters: {
          'environment/hasPermission': (permission) =>
            permissions.includes(permission),
        },
        dispatch,
      },
      $message: { error: vi.fn(), success: vi.fn() },
      $confirm: vi.fn(),
    },
  })
  return { dispatch, wrapper }
}

function importContext() {
  return {
    $message: { error: vi.fn(), success: vi.fn() },
    $refs: { importForm: { validate: (done) => done(true) } },
    environmentId: 11,
    importDialog: {
      visible: true,
      saving: false,
      sourceSnapshotId: 31,
      targetHasPermission: true,
      errorMessage: '',
      form: {
        targetEnvironmentId: 22,
        targetProjectId: 7,
        targetConfigId: 9,
        conflictStrategy: 'REPLACE',
        description: '同步快照',
        tags: ['REUSE'],
        idempotencyKey: 'kirby-import-fixed-12345',
      },
    },
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  transferApi.createImportIdempotencyKey.mockReturnValue(
    'kirby-import-fixed-12345',
  )
  transferApi.publishSnapshot.mockResolvedValue({ snapshot: {} })
  transferApi.exportSnapshot.mockResolvedValue({ snapshot: {} })
  transferApi.importSnapshot.mockResolvedValue({ snapshot: {}, replayed: false })
  environmentApi.getMyPermissions.mockResolvedValue({
    permissions: ['snapshot:import', 'config:write'],
  })
})

describe('snapshot states and permissions', () => {
  it('accepts only unreleased and released states', () => {
    expect(snapshotStatusLabel('UNRELEASED')).toBe('未发布')
    expect(snapshotStatusLabel('RELEASED')).toBe('已发布')
    expect(() => snapshotStatusLabel('PENDING')).toThrow(
      '不支持的快照状态: PENDING',
    )
    expect(() =>
      normalizeSnapshotList({
        list: [{ status: 'REJECTED', tags: [] }],
        page: {},
      }),
    ).toThrow('不支持的快照状态: REJECTED')
  })

  it('exposes publication only with the backend permission', async () => {
    const denied = mountPanel(['snapshot:read'])
    await flush()
    expect(denied.wrapper.vm.canPublish).toBe(false)
    await expect(
      denied.wrapper.vm.publish({ id: 31, version: 2 }),
    ).rejects.toThrow('当前用户没有发布快照权限')
    expect(transferApi.publishSnapshot).not.toHaveBeenCalled()

    const allowed = mountPanel(['snapshot:read', 'snapshot:publish'])
    await flush()
    allowed.wrapper.vm.reload = vi.fn().mockResolvedValue()
    await allowed.wrapper.vm.publish({ id: 31, version: 2 })
    expect(transferApi.publishSnapshot).toHaveBeenCalledWith(11, 31, 2)
  })
})

describe('snapshot import authorization and retry', () => {
  it('reuses one idempotency key after a network failure', async () => {
    const context = importContext()
    transferApi.importSnapshot
      .mockRejectedValueOnce(new Error('network unavailable'))
      .mockResolvedValueOnce({ snapshot: { id: 91 }, replayed: true })

    await SnapshotsPanel.methods.submitImport.call(context)
    await SnapshotsPanel.methods.submitImport.call(context)

    expect(transferApi.importSnapshot).toHaveBeenCalledTimes(2)
    expect(
      transferApi.importSnapshot.mock.calls.map(
        ([, request]) => request.idempotency_key,
      ),
    ).toEqual([
      'kirby-import-fixed-12345',
      'kirby-import-fixed-12345',
    ])
  })

  it('uses a new key after business fields change and then keeps it stable', async () => {
    const context = importContext()
    transferApi.createImportIdempotencyKey.mockReturnValue(
      'kirby-import-new-request-67890',
    )
    transferApi.importSnapshot
      .mockRejectedValueOnce(new Error('first request failed'))
      .mockRejectedValueOnce(new Error('changed request failed'))
      .mockResolvedValueOnce({ snapshot: { id: 91 }, replayed: true })

    await SnapshotsPanel.methods.submitImport.call(context)
    context.importDialog.form.targetProjectId = 8
    await SnapshotsPanel.methods.submitImport.call(context)
    await SnapshotsPanel.methods.submitImport.call(context)

    expect(
      transferApi.importSnapshot.mock.calls.map(
        ([, request]) => request.idempotency_key,
      ),
    ).toEqual([
      'kirby-import-fixed-12345',
      'kirby-import-new-request-67890',
      'kirby-import-new-request-67890',
    ])
  })

  it('requires both import and config write permissions in the target environment', async () => {
    environmentApi.getMyPermissions.mockResolvedValue({
      permissions: ['snapshot:import'],
    })
    const context = {
      $store: { dispatch: vi.fn() },
      importDialog: {
        loadingTarget: false,
        errorMessage: '',
        targetHasPermission: false,
        projects: [],
        configs: [],
        form: { targetProjectId: null, targetConfigId: null },
      },
    }

    await SnapshotsPanel.methods.loadTargetEnvironment.call(context, 22)

    expect(context.importDialog.targetHasPermission).toBe(false)
    expect(context.importDialog.errorMessage).toBe(
      '当前用户没有目标环境的快照导入或配置写入权限。',
    )
    expect(context.$store.dispatch).not.toHaveBeenCalled()
  })

  it('shows an explicit source permission error after export returns 403', async () => {
    transferApi.exportSnapshot.mockRejectedValue({ response: { status: 403 } })
    const context = {
      canExport: true,
      environmentId: 11,
      errorMessage: '',
      $message: { error: vi.fn() },
    }

    await SnapshotsPanel.methods.download.call(context, { id: 31 })

    expect(context.errorMessage).toBe(
      '没有权限导出源快照。当前页面已保留。',
    )
  })

  it('shows an explicit target permission error and keeps the form', async () => {
    environmentApi.getMyPermissions.mockRejectedValue({
      response: { status: 403 },
    })
    const context = {
      $store: { dispatch: vi.fn() },
      importDialog: {
        loadingTarget: false,
        errorMessage: '',
        targetHasPermission: true,
        projects: [{ id: 1 }],
        configs: [{ id: 2 }],
        form: { targetProjectId: 1, targetConfigId: 2 },
      },
    }

    await SnapshotsPanel.methods.loadTargetEnvironment.call(context, 22)

    expect(context.importDialog.errorMessage).toBe(
      '没有权限读取目标环境权限和项目。当前导入表单已保留。',
    )
    expect(context.importDialog.form).toMatchObject({
      targetProjectId: null,
      targetConfigId: null,
    })
  })

  it('keeps the retry form after backend authorization changes', async () => {
    transferApi.importSnapshot.mockRejectedValue({
      response: { status: 403 },
    })
    const context = importContext()

    await SnapshotsPanel.methods.submitImport.call(context)

    expect(context.importDialog.visible).toBe(true)
    expect(context.importDialog.form.idempotencyKey).toBe(
      'kirby-import-fixed-12345',
    )
    expect(context.importDialog.errorMessage).toBe(
      '没有权限从源环境导出或向目标环境导入。当前导入表单已保留，可重试同一请求。',
    )
  })
})
