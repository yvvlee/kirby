<template>
  <el-dialog
    :visible="visible"
    title="项目 API Key"
    width="900px"
    append-to-body
    :close-on-click-modal="false"
    @update:visible="$emit('update:visible', $event)"
  >
    <el-alert
      v-if="errorMessage"
      :title="errorMessage"
      type="error"
      :closable="false"
      show-icon
    />

    <header class="api-keys-panel__toolbar">
      <p>Secret 只在创建或轮换后显示一次。</p>
      <el-button
        v-if="canManage"
        type="primary"
        size="small"
        @click="openCreate"
      >
        创建 API Key
      </el-button>
    </header>

    <el-table
      v-loading="loading"
      :data="keys"
      stripe
      empty-text="当前项目还没有 API Key"
    >
      <el-table-column
        prop="name"
        label="名称"
        min-width="150"
      />
      <el-table-column
        prop="publicId"
        label="公开 ID"
        min-width="180"
      />
      <el-table-column
        label="Secret 尾号"
        width="120"
      >
        <template #default="{ row }">
          ****{{ row.secretSuffix }}
        </template>
      </el-table-column>
      <el-table-column
        prop="lastUsedAt"
        label="最后使用"
        width="180"
      >
        <template #default="{ row }">
          {{ row.lastUsedAt || '从未使用' }}
        </template>
      </el-table-column>
      <el-table-column
        prop="createdAt"
        label="创建时间"
        width="180"
      />
      <el-table-column
        v-if="canManage"
        label="操作"
        width="150"
        align="right"
      >
        <template #default="{ row }">
          <template v-if="!row.revokedAt">
            <el-button
              type="text"
              @click="rotate(row)"
            >
              轮换
            </el-button>
            <el-button
              class="api-keys-panel__danger"
              type="text"
              @click="revoke(row)"
            >
              吊销
            </el-button>
          </template>
          <el-tag
            v-else
            type="info"
            size="small"
          >
            已吊销
          </el-tag>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      :visible.sync="createDialog.visible"
      title="创建 API Key"
      width="480px"
      append-to-body
      :close-on-click-modal="false"
    >
      <el-form
        v-if="createDialog.visible"
        ref="createForm"
        :model="createDialog.form"
        :rules="createRules"
        label-width="80px"
      >
        <el-form-item
          label="名称"
          prop="name"
        >
          <el-input
            v-model.trim="createDialog.form.name"
            maxlength="64"
            placeholder="例如 production"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialog.visible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="createDialog.saving"
          @click="createKey"
        >
          创建
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      :visible.sync="secretDialog.visible"
      title="立即复制 API Key Secret"
      width="620px"
      append-to-body
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      @closed="clearSecret"
    >
      <el-alert
        title="关闭后无法再次查看完整 Secret。请立即复制并安全保存。"
        type="warning"
        :closable="false"
        show-icon
      />
      <div class="api-keys-panel__secret">
        <code>{{ secretDialog.secret }}</code>
        <el-button
          type="primary"
          plain
          @click="copySecret"
        >
          {{ secretDialog.copied ? '已复制' : '复制' }}
        </el-button>
      </div>
      <el-checkbox v-model="secretDialog.acknowledged">
        我确认已经复制并保存 Secret
      </el-checkbox>
      <template #footer>
        <el-button
          type="primary"
          :disabled="!secretDialog.acknowledged"
          @click="confirmSecretCopied"
        >
          确认并清除
        </el-button>
      </template>
    </el-dialog>
  </el-dialog>
</template>

<script>
import {
  createProjectApiKey,
  listProjectApiKeys,
  revokeProjectApiKey,
  rotateProjectApiKey,
} from '@/api/api-keys'

function actionError(error, action) {
  if (error?.response?.status === 403) {
    return `没有权限${action}。当前页面已保留。`
  }
  return error?.response?.data?.message || error?.message || `${action}失败`
}

function emptyCreateDialog() {
  return {
    visible: false,
    saving: false,
    form: { name: '' },
  }
}

function emptySecretDialog() {
  return {
    visible: false,
    secret: null,
    copied: false,
    acknowledged: false,
  }
}

function requireKeyList(reply) {
  if (!Array.isArray(reply?.list)) {
    throw new TypeError('API Key 列表响应缺少 list')
  }
  reply.list.forEach((item) => {
    if (!item || typeof item !== 'object') {
      throw new TypeError('API Key 列表项必须是对象')
    }
    if (Object.hasOwn(item, 'secret')) {
      throw new Error('API Key 列表不得包含完整 Secret')
    }
  })
  return reply.list
}

function requireSecretReply(reply) {
  if (!reply?.apiKey || typeof reply.secret !== 'string' || !reply.secret) {
    throw new TypeError('API Key 响应缺少一次性 Secret')
  }
  return reply.secret
}

export default {
  name: 'ApiKeysPanel',

  props: {
    visible: { type: Boolean, required: true },
    projectId: { type: Number, required: true },
  },

  data() {
    return {
      loading: false,
      errorMessage: '',
      keys: [],
      createDialog: emptyCreateDialog(),
      secretDialog: emptySecretDialog(),
      createRules: {
        name: [
          { required: true, message: '请输入 API Key 名称', trigger: 'blur' },
          { max: 64, message: '名称最多 64 个字符', trigger: 'blur' },
        ],
      },
    }
  },

  computed: {
    environmentId() {
      return this.$store.state.environment.currentId
    },
    canManage() {
      return this.$store.getters['environment/hasPermission'](
        'project:api_key:manage',
      )
    },
  },

  watch: {
    visible: {
      immediate: true,
      handler(visible) {
        if (visible) {
          this.reload()
        }
      },
    },
    projectId() {
      if (this.visible) {
        this.reload()
      }
    },
  },

  beforeDestroy() {
    this.clearSecret()
  },

  methods: {
    async reload() {
      this.loading = true
      this.errorMessage = ''
      try {
        const reply = await listProjectApiKeys(
          this.environmentId,
          this.projectId,
        )
        this.keys = requireKeyList(reply)
      } catch (error) {
        this.errorMessage = actionError(error, '读取项目 API Key')
        this.$message.error(this.errorMessage)
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      if (!this.canManage) {
        throw new Error('当前用户没有管理项目 API Key 的权限')
      }
      this.createDialog = { ...emptyCreateDialog(), visible: true }
    },
    showSecret(reply) {
      this.secretDialog = {
        ...emptySecretDialog(),
        visible: true,
        secret: requireSecretReply(reply),
      }
    },
    async createKey() {
      const valid = await new Promise((resolve) => {
        this.$refs.createForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      this.createDialog.saving = true
      try {
        const reply = await createProjectApiKey(
          this.environmentId,
          this.projectId,
          this.createDialog.form.name,
        )
        this.createDialog.visible = false
        this.showSecret(reply)
        await this.reload()
      } catch (error) {
        this.errorMessage = actionError(error, '创建项目 API Key')
        this.$message.error(this.errorMessage)
      } finally {
        this.createDialog.saving = false
      }
    },
    async rotate(row) {
      if (!this.canManage) {
        throw new Error('当前用户没有管理项目 API Key 的权限')
      }
      try {
        await this.$confirm(
          `轮换“${row.name}”后旧 Secret 会立即失效，是否继续？`,
          '轮换 API Key',
          { type: 'warning', confirmButtonText: '轮换', cancelButtonText: '取消' },
        )
      } catch (error) {
        if (error === 'cancel' || error === 'close') {
          return
        }
        throw error
      }
      try {
        const reply = await rotateProjectApiKey(
          this.environmentId,
          this.projectId,
          row.id,
        )
        this.showSecret(reply)
        await this.reload()
      } catch (error) {
        this.errorMessage = actionError(error, '轮换项目 API Key')
        this.$message.error(this.errorMessage)
      }
    },
    async revoke(row) {
      if (!this.canManage) {
        throw new Error('当前用户没有管理项目 API Key 的权限')
      }
      try {
        await this.$confirm(
          `吊销“${row.name}”后客户端会立即无法访问，是否继续？`,
          '吊销 API Key',
          { type: 'warning', confirmButtonText: '吊销', cancelButtonText: '取消' },
        )
      } catch (error) {
        if (error === 'cancel' || error === 'close') {
          return
        }
        throw error
      }
      try {
        await revokeProjectApiKey(
          this.environmentId,
          this.projectId,
          row.id,
        )
        this.$message.success('API Key 已吊销')
        await this.reload()
      } catch (error) {
        this.errorMessage = actionError(error, '吊销项目 API Key')
        this.$message.error(this.errorMessage)
      }
    },
    async copySecret() {
      if (!this.secretDialog.secret) {
        throw new Error('一次性 Secret 已清除')
      }
      if (typeof navigator.clipboard?.writeText !== 'function') {
        throw new Error('当前浏览器不支持安全剪贴板写入')
      }
      await navigator.clipboard.writeText(this.secretDialog.secret)
      this.secretDialog.copied = true
      this.$message.success('Secret 已复制')
    },
    confirmSecretCopied() {
      if (!this.secretDialog.acknowledged) {
        throw new Error('必须确认已经复制并保存 Secret')
      }
      this.secretDialog.visible = false
    },
    clearSecret() {
      this.secretDialog = emptySecretDialog()
    },
  },
}
</script>

<style scoped>
.api-keys-panel__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 16px 0;
}

.api-keys-panel__toolbar p {
  margin: 0;
  color: #6b7280;
}

.api-keys-panel__secret {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 20px 0;
}

.api-keys-panel__secret code {
  flex: 1;
  overflow-wrap: anywhere;
  padding: 12px;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  background: #f8fafc;
  user-select: all;
}

.api-keys-panel__danger {
  color: #f56c6c;
}
</style>
