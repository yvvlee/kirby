<template>
  <section aria-label="快照管理">
    <el-alert
      v-if="errorMessage"
      :title="errorMessage"
      type="error"
      :closable="false"
      show-icon
    />

    <header class="snapshots-panel__toolbar">
      <p>快照用于比较、还原、发布和跨环境复用配置。</p>
      <div>
        <el-button
          size="small"
          :loading="loading"
          @click="reload"
        >
          刷新
        </el-button>
        <el-button
          v-if="canWrite"
          type="primary"
          size="small"
          @click="openCreate"
        >
          创建快照
        </el-button>
      </div>
    </header>

    <el-table
      v-loading="loading"
      :data="snapshots"
      stripe
      empty-text="当前配置还没有快照"
    >
      <el-table-column
        prop="id"
        label="ID"
        width="80"
      />
      <el-table-column
        prop="description"
        label="描述"
        min-width="220"
      />
      <el-table-column
        label="标签"
        min-width="180"
      >
        <template #default="{ row }">
          <el-tag
            v-for="tag in row.tags"
            :key="tag"
            class="snapshots-panel__tag"
            size="mini"
            type="info"
          >
            {{ tag }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column
        label="状态"
        width="110"
      >
        <template #default="{ row }">
          <el-tag
            :type="row.status === 'RELEASED' ? 'success' : 'info'"
            size="small"
          >
            {{ statusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column
        prop="createdBy"
        label="创建人"
        width="130"
      />
      <el-table-column
        prop="createdAt"
        label="创建时间"
        width="190"
      />
      <el-table-column
        label="操作"
        min-width="430"
        align="right"
      >
        <template #default="{ row }">
          <el-button
            type="text"
            @click="openCompare(row)"
          >
            比较
          </el-button>
          <el-button
            v-if="canWrite"
            type="text"
            @click="restore(row)"
          >
            还原
          </el-button>
          <el-button
            v-if="canPublish && row.status === 'UNRELEASED'"
            type="text"
            @click="publish(row)"
          >
            发布
          </el-button>
          <el-button
            v-if="canPublish && row.status === 'RELEASED'"
            type="text"
            @click="unpublish(row)"
          >
            下线
          </el-button>
          <el-button
            v-if="canExport"
            type="text"
            @click="download(row)"
          >
            导出
          </el-button>
          <el-button
            v-if="canExport"
            type="text"
            @click="openImport(row)"
          >
            导入到环境
          </el-button>
          <el-button
            v-if="canWrite && row.status === 'UNRELEASED'"
            class="snapshots-panel__danger"
            type="text"
            @click="remove(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      class="snapshots-panel__pagination"
      :current-page="page.page"
      :page-size="page.limit"
      :page-sizes="[10, 20, 50, 100]"
      :total="page.total"
      layout="total, sizes, prev, pager, next"
      @current-change="changePage"
      @size-change="changePageSize"
    />

    <el-dialog
      :visible.sync="createDialog.visible"
      title="创建快照"
      width="1100px"
      append-to-body
      :close-on-click-modal="false"
      @closed="clearCreate"
    >
      <el-form
        v-if="createDialog.visible"
        ref="createForm"
        :model="createDialog.form"
        :rules="createRules"
        label-width="88px"
      >
        <el-form-item
          label="快照标签"
          prop="tags"
        >
          <el-select
            v-model="createDialog.form.tags"
            multiple
            size="small"
          >
            <el-option
              v-for="tag in tagOptions"
              :key="tag.value"
              :label="tag.label"
              :value="tag.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item
          label="快照描述"
          prop="description"
        >
          <el-input
            v-model="createDialog.form.description"
            type="textarea"
            :rows="3"
            maxlength="255"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="配置变化">
          <p class="snapshots-panel__hint">
            左侧是当前配置，右侧是待创建快照。
          </p>
          <DiffEditor
            v-if="createDialog.previewReady"
            class="snapshots-panel__diff"
            :left-value="createDialog.currentContent"
            :right-value="createDialog.previewContent"
          />
          <div v-else>
            正在读取快照预览…
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialog.visible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="createDialog.saving"
          :disabled="!createDialog.previewReady"
          @click="createSnapshotRecord"
        >
          创建
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      :visible.sync="compareDialog.visible"
      title="快照内容比较"
      width="1100px"
      append-to-body
      @closed="clearCompare"
    >
      <p class="snapshots-panel__hint">
        左侧是当前配置，右侧是所选快照。
      </p>
      <DiffEditor
        v-if="compareDialog.ready"
        class="snapshots-panel__diff"
        :left-value="compareDialog.currentContent"
        :right-value="compareDialog.snapshotContent"
      />
      <div v-else>
        正在读取比较内容…
      </div>
    </el-dialog>

    <el-dialog
      :visible.sync="importDialog.visible"
      title="导入快照到目标环境"
      width="680px"
      append-to-body
      :close-on-click-modal="false"
      @closed="clearImport"
    >
      <el-alert
        v-if="importDialog.errorMessage"
        :title="importDialog.errorMessage"
        type="error"
        :closable="false"
        show-icon
      />
      <el-form
        v-if="importDialog.visible"
        ref="importForm"
        :model="importDialog.form"
        :rules="importRules"
        label-width="112px"
      >
        <el-form-item label="源快照">
          <el-input
            :value="`环境 ${environmentId} / 快照 ${importDialog.sourceSnapshotId}`"
            disabled
          />
        </el-form-item>
        <el-form-item
          label="目标环境"
          prop="targetEnvironmentId"
        >
          <el-select
            v-model="importDialog.form.targetEnvironmentId"
            filterable
            @change="loadTargetEnvironment"
          >
            <el-option
              v-for="environment in availableEnvironments"
              :key="String(environment.id)"
              :label="environment.name"
              :value="environment.id"
              :disabled="!environment.enabled"
            />
          </el-select>
        </el-form-item>
        <el-form-item
          label="目标项目"
          prop="targetProjectId"
        >
          <el-select
            v-model="importDialog.form.targetProjectId"
            filterable
            :disabled="!importDialog.targetHasPermission"
            @change="loadTargetConfigs"
          >
            <el-option
              v-for="project in importDialog.projects"
              :key="String(project.id)"
              :label="project.name"
              :value="project.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item
          label="冲突策略"
          prop="conflictStrategy"
        >
          <el-select v-model="importDialog.form.conflictStrategy">
            <el-option
              label="冲突时报错并创建新配置"
              value="FAIL"
            />
            <el-option
              label="替换指定配置"
              value="REPLACE"
            />
          </el-select>
        </el-form-item>
        <el-form-item
          v-if="importDialog.form.conflictStrategy === 'REPLACE'"
          label="目标配置"
          prop="targetConfigId"
        >
          <el-select
            v-model="importDialog.form.targetConfigId"
            filterable
          >
            <el-option
              v-for="config in importDialog.configs"
              :key="String(config.id)"
              :label="config.description || config.key"
              :value="config.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item
          label="快照描述"
          prop="description"
        >
          <el-input
            v-model="importDialog.form.description"
            maxlength="255"
          />
        </el-form-item>
        <el-form-item label="快照标签">
          <el-select
            v-model="importDialog.form.tags"
            multiple
          >
            <el-option
              v-for="tag in tagOptions"
              :key="tag.value"
              :label="tag.label"
              :value="tag.value"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="importDialog.visible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="importDialog.saving"
          :disabled="!importDialog.targetHasPermission"
          @click="submitImport"
        >
          导入
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import {
  createSnapshot,
  deleteSnapshot,
  getCurrentSnapshot,
  getSnapshot,
  loadSnapshot,
  previewCreatingSnapshot,
} from '@/api/snapshots'
import {
  createImportIdempotencyKey,
  exportSnapshot,
  importSnapshot,
  publishSnapshot,
  unpublishSnapshot,
} from '@/api/snapshot-imports'
import { getMyPermissions } from '@/api/environments'
import DiffEditor from '@/components/DiffEditor'
import {
  normalizeSnapshotList,
  parseSnapshotContent,
  SNAPSHOT_TAG_OPTIONS,
  snapshotActionError,
  snapshotStatusLabel,
} from './model'

function emptyCreateDialog() {
  return {
    visible: false,
    saving: false,
    previewReady: false,
    currentContent: null,
    previewContent: null,
    form: { description: '', tags: [] },
  }
}

function emptyCompareDialog() {
  return {
    visible: false,
    ready: false,
    currentContent: null,
    snapshotContent: null,
  }
}

function emptyImportDialog() {
  return {
    visible: false,
    saving: false,
    loadingTarget: false,
    sourceSnapshotId: null,
    requestSignature: '',
    targetHasPermission: false,
    projects: [],
    configs: [],
    errorMessage: '',
    form: {
      targetEnvironmentId: null,
      targetProjectId: null,
      targetConfigId: null,
      conflictStrategy: 'FAIL',
      description: '',
      tags: [],
      idempotencyKey: '',
    },
  }
}

function requireSnapshot(reply, action) {
  if (!reply?.snapshot) {
    throw new TypeError(`${action}响应缺少 snapshot`)
  }
  return reply.snapshot
}

function configRows(reply) {
  if (!Array.isArray(reply?.list)) {
    throw new TypeError('目标配置列表响应缺少 list')
  }
  return reply.list.map((item) => {
    if (!item?.config) {
      throw new TypeError('目标配置列表项缺少 config')
    }
    return item.config
  })
}

export default {
  name: 'SnapshotsPanel',

  components: { DiffEditor },

  props: {
    projectId: { type: Number, required: true },
    configId: { type: Number, required: true },
  },

  data() {
    return {
      errorMessage: '',
      loading: false,
      snapshots: [],
      page: { page: 1, limit: 10, total: 0 },
      tagOptions: SNAPSHOT_TAG_OPTIONS,
      createDialog: emptyCreateDialog(),
      compareDialog: emptyCompareDialog(),
      importDialog: emptyImportDialog(),
      createRules: {
        description: [
          { required: true, message: '请输入快照描述', trigger: 'blur' },
          { min: 2, max: 255, message: '快照描述长度为 2 到 255 个字符', trigger: 'blur' },
        ],
        tags: [
          { type: 'array', required: true, min: 1, message: '请选择快照标签', trigger: 'change' },
        ],
      },
      importRules: {
        targetEnvironmentId: [
          { required: true, message: '请选择目标环境', trigger: 'change' },
        ],
        targetProjectId: [
          { required: true, message: '请选择目标项目', trigger: 'change' },
        ],
        targetConfigId: [
          { required: true, message: '请选择目标配置', trigger: 'change' },
        ],
        conflictStrategy: [
          { required: true, message: '请选择冲突策略', trigger: 'change' },
        ],
        description: [
          { required: true, message: '请输入快照描述', trigger: 'blur' },
          { min: 2, max: 255, message: '快照描述长度为 2 到 255 个字符', trigger: 'blur' },
        ],
      },
    }
  },

  computed: {
    environmentId() {
      return this.$store.state.environment.currentId
    },
    availableEnvironments() {
      return this.$store.state.environment.available
    },
    canWrite() {
      return this.$store.getters['environment/hasPermission']('snapshot:write')
    },
    canPublish() {
      return this.$store.getters['environment/hasPermission']('snapshot:publish')
    },
    canExport() {
      return this.$store.getters['environment/hasPermission']('snapshot:export')
    },
  },

  watch: {
    configId() {
      this.page.page = 1
      this.reload()
    },
  },

  created() {
    this.reload()
  },

  methods: {
    statusLabel: snapshotStatusLabel,
    async reload() {
      this.loading = true
      this.errorMessage = ''
      try {
        const reply = await this.$store.dispatch('configCenter/loadSnapshots', {
          environmentId: this.environmentId,
          projectId: this.projectId,
          configId: this.configId,
          filter: { page: { page: this.page.page, limit: this.page.limit } },
          force: true,
        })
        const normalized = normalizeSnapshotList(reply)
        this.snapshots = normalized.list
        this.page = {
          page: normalized.page.page,
          limit: normalized.page.limit,
          total: normalized.page.total,
        }
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '读取快照')
        this.$message.error(this.errorMessage)
      } finally {
        this.loading = false
      }
    },
    changePage(page) {
      this.page.page = page
      return this.reload()
    },
    changePageSize(limit) {
      this.page.page = 1
      this.page.limit = limit
      return this.reload()
    },
    async openCreate() {
      if (!this.canWrite) {
        throw new Error('当前用户没有创建快照权限')
      }
      this.createDialog = { ...emptyCreateDialog(), visible: true }
      try {
        const [currentReply, previewReply] = await Promise.all([
          getCurrentSnapshot(this.environmentId, this.configId),
          previewCreatingSnapshot(this.environmentId, this.configId),
        ])
        const current = requireSnapshot(currentReply, '当前快照')
        this.createDialog.currentContent = parseSnapshotContent(current.content)
        this.createDialog.previewContent = parseSnapshotContent(previewReply.content)
        this.createDialog.previewReady = true
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '读取快照预览')
        this.$message.error(this.errorMessage)
      }
    },
    clearCreate() {
      this.createDialog = emptyCreateDialog()
    },
    async createSnapshotRecord() {
      const valid = await new Promise((resolve) => {
        this.$refs.createForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      this.createDialog.saving = true
      try {
        await createSnapshot(this.environmentId, {
          config_id: this.configId,
          project_id: this.projectId,
          description: this.createDialog.form.description,
          tags: this.createDialog.form.tags,
        })
        this.createDialog.visible = false
        this.$message.success('快照已创建')
        await this.reload()
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '创建快照')
        this.$message.error(this.errorMessage)
      } finally {
        this.createDialog.saving = false
      }
    },
    async openCompare(row) {
      this.compareDialog = { ...emptyCompareDialog(), visible: true }
      try {
        const [currentReply, snapshotReply] = await Promise.all([
          getCurrentSnapshot(this.environmentId, this.configId),
          getSnapshot(this.environmentId, row.id),
        ])
        const current = requireSnapshot(currentReply, '当前快照')
        const snapshot = requireSnapshot(snapshotReply, '快照详情')
        this.compareDialog.currentContent = parseSnapshotContent(current.content)
        this.compareDialog.snapshotContent = parseSnapshotContent(snapshot.content)
        this.compareDialog.ready = true
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '比较快照')
        this.$message.error(this.errorMessage)
      }
    },
    clearCompare() {
      this.compareDialog = emptyCompareDialog()
    },
    async restore(row) {
      if (!this.canWrite) {
        throw new Error('当前用户没有还原快照权限')
      }
      try {
        await this.$confirm('还原会覆盖尚未保存为快照的配置，是否继续？', '还原快照', {
          type: 'warning',
          confirmButtonText: '还原',
          cancelButtonText: '取消',
        })
      } catch (error) {
        if (error === 'cancel' || error === 'close') {
          return
        }
        throw error
      }
      try {
        await loadSnapshot(this.environmentId, row.id, this.configId)
        this.$message.success('快照已还原')
        this.$emit('changed')
        await this.reload()
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '还原快照')
        this.$message.error(this.errorMessage)
      }
    },
    async publish(row) {
      if (!this.canPublish) {
        throw new Error('当前用户没有发布快照权限')
      }
      try {
        await publishSnapshot(this.environmentId, row.id, row.version)
        this.$message.success('快照已发布')
        await this.reload()
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '发布快照')
        this.$message.error(this.errorMessage)
      }
    },
    async unpublish(row) {
      if (!this.canPublish) {
        throw new Error('当前用户没有下线快照权限')
      }
      try {
        await unpublishSnapshot(this.environmentId, row.id, row.version)
        this.$message.success('快照已下线')
        await this.reload()
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '下线快照')
        this.$message.error(this.errorMessage)
      }
    },
    async download(row) {
      if (!this.canExport) {
        throw new Error('当前用户没有导出快照权限')
      }
      try {
        const reply = await exportSnapshot(this.environmentId, row.id)
        const blob = new Blob([JSON.stringify(reply, null, 2)], {
          type: 'application/json',
        })
        const url = URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = `kirby-snapshot-${row.id}.json`
        link.click()
        URL.revokeObjectURL(url)
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '导出源快照')
        this.$message.error(this.errorMessage)
      }
    },
    openImport(row) {
      if (!this.canExport) {
        throw new Error('当前用户没有导出源快照权限')
      }
      this.importDialog = {
        ...emptyImportDialog(),
        visible: true,
        sourceSnapshotId: row.id,
        form: {
          ...emptyImportDialog().form,
          description: row.description,
          tags: [...row.tags],
          idempotencyKey: createImportIdempotencyKey(),
        },
      }
    },
    clearImport() {
      this.importDialog = emptyImportDialog()
    },
    async loadTargetEnvironment(targetEnvironmentId) {
      this.importDialog.loadingTarget = true
      this.importDialog.errorMessage = ''
      this.importDialog.targetHasPermission = false
      this.importDialog.projects = []
      this.importDialog.configs = []
      this.importDialog.form.targetProjectId = null
      this.importDialog.form.targetConfigId = null
      try {
        const permissionReply = await getMyPermissions(targetEnvironmentId)
        if (!Array.isArray(permissionReply?.permissions)) {
          throw new TypeError('目标环境权限响应缺少 permissions')
        }
        const targetPermissions = new Set(permissionReply.permissions)
        if (
          !targetPermissions.has('snapshot:import') ||
          !targetPermissions.has('config:write')
        ) {
          this.importDialog.errorMessage =
            '当前用户没有目标环境的快照导入或配置写入权限。'
          return
        }
        this.importDialog.targetHasPermission = true
        const projectReply = await this.$store.dispatch('configCenter/loadProjects', {
          environmentId: targetEnvironmentId,
          filter: {},
          force: true,
        })
        if (!Array.isArray(projectReply?.list)) {
          throw new TypeError('目标项目列表响应缺少 list')
        }
        this.importDialog.projects = projectReply.list
      } catch (error) {
        this.importDialog.errorMessage = snapshotActionError(
          error,
          '读取目标环境权限和项目',
          '当前导入表单已保留。',
        )
      } finally {
        this.importDialog.loadingTarget = false
      }
    },
    async loadTargetConfigs(targetProjectId) {
      this.importDialog.configs = []
      this.importDialog.form.targetConfigId = null
      if (!targetProjectId) {
        return
      }
      try {
        const reply = await this.$store.dispatch('configCenter/loadConfigs', {
          environmentId: this.importDialog.form.targetEnvironmentId,
          projectId: targetProjectId,
          filter: {},
          force: true,
        })
        this.importDialog.configs = configRows(reply)
      } catch (error) {
        this.importDialog.errorMessage = snapshotActionError(
          error,
          '读取目标配置',
          '当前导入表单已保留。',
        )
      }
    },
    async submitImport() {
      if (!this.importDialog.targetHasPermission) {
        throw new Error('目标环境没有快照导入权限')
      }
      const valid = await new Promise((resolve) => {
        this.$refs.importForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      const form = this.importDialog.form
      if (form.conflictStrategy === 'REPLACE' && !form.targetConfigId) {
        this.importDialog.errorMessage = '替换策略必须选择目标配置。'
        return
      }
      this.importDialog.saving = true
      this.importDialog.errorMessage = ''
      try {
        const request = {
          source_environment_id: this.environmentId,
          source_snapshot_id: this.importDialog.sourceSnapshotId,
          target_project_id: form.targetProjectId,
          description: form.description,
          tags: form.tags,
          conflict_strategy: form.conflictStrategy,
        }
        if (form.conflictStrategy === 'REPLACE') {
          request.target_config_id = form.targetConfigId
        }
        const requestSignature = JSON.stringify({
          ...request,
          target_environment_id: form.targetEnvironmentId,
        })
        if (
          this.importDialog.requestSignature &&
          this.importDialog.requestSignature !== requestSignature
        ) {
          form.idempotencyKey = createImportIdempotencyKey()
        }
        this.importDialog.requestSignature = requestSignature
        request.idempotency_key = form.idempotencyKey
        const reply = await importSnapshot(form.targetEnvironmentId, request)
        if (!reply?.snapshot) {
          throw new TypeError('快照导入响应缺少 snapshot')
        }
        this.$message.success(reply.replayed ? '已返回上次导入结果' : '快照已导入')
        this.importDialog.visible = false
      } catch (error) {
        this.importDialog.errorMessage = snapshotActionError(
          error,
          '从源环境导出或向目标环境导入',
          '当前导入表单已保留，可重试同一请求。',
        )
        this.$message.error(this.importDialog.errorMessage)
      } finally {
        this.importDialog.saving = false
      }
    },
    async remove(row) {
      if (!this.canWrite) {
        throw new Error('当前用户没有删除快照权限')
      }
      try {
        await this.$confirm(`确认删除快照“${row.description}”吗？`, '删除快照', {
          type: 'warning',
          confirmButtonText: '删除',
          cancelButtonText: '取消',
        })
      } catch (error) {
        if (error === 'cancel' || error === 'close') {
          return
        }
        throw error
      }
      try {
        await deleteSnapshot(this.environmentId, row.id)
        this.$message.success('快照已删除')
        await this.reload()
      } catch (error) {
        this.errorMessage = snapshotActionError(error, '删除快照')
        this.$message.error(this.errorMessage)
      }
    },
  },
}
</script>

<style scoped>
.snapshots-panel__toolbar,
.snapshots-panel__pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 16px 0;
}

.snapshots-panel__toolbar p,
.snapshots-panel__hint {
  margin: 0;
  color: #6b7280;
}

.snapshots-panel__pagination {
  justify-content: flex-end;
}

.snapshots-panel__tag {
  margin: 2px 4px 2px 0;
}

.snapshots-panel__diff {
  min-height: 420px;
  margin-top: 10px;
}

.snapshots-panel__danger {
  color: #f56c6c;
}
</style>
