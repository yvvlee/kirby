<template>
  <section
    v-loading="loading"
    class="config-detail"
    aria-labelledby="config-detail-title"
  >
    <el-page-header @back="goBack">
      <template #content>
        <div class="config-detail__title">
          <span id="config-detail-title">
            {{ config ? config.key : '配置详情' }}
          </span>
          <EnvironmentTag :environment="environment" />
        </div>
      </template>
    </el-page-header>

    <el-card
      v-if="config"
      shadow="never"
    >
      <header class="config-detail__summary-header">
        <div>
          <h1>{{ config.description || config.key }}</h1>
          <p>{{ project ? project.name : '' }} · {{ config.key }}</p>
        </div>
        <el-button
          v-if="canWriteConfig"
          type="primary"
          size="small"
          @click="openConfigEditor"
        >
          修改配置定义
        </el-button>
      </header>
      <el-descriptions
        :column="3"
        border
        size="small"
      >
        <el-descriptions-item label="数据类型">
          {{ typeLabel }}
        </el-descriptions-item>
        <el-descriptions-item label="数组">
          {{ config.isArray ? '是' : '否' }}
        </el-descriptions-item>
        <el-descriptions-item label="版本">
          {{ config.version }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card
      v-if="config"
      class="config-detail__tabs"
      shadow="never"
    >
      <el-tabs v-model="activeTab">
        <el-tab-pane
          label="配置内容"
          name="content"
        >
          <div class="config-detail__editor-grid">
            <div class="config-detail__form-pane">
              <SchemaForm
                v-if="tree"
                ref="schemaForm"
                :config="tree"
                :value="config.value || ''"
                :disabled="!canWriteConfig"
                :models="models"
                :enums="enums"
                :file-upload-component="fileFieldUnavailable"
              />
              <el-alert
                v-else
                type="warning"
                :closable="false"
                title="配置结构不可用，请先修改配置定义。"
              />
            </div>
            <div class="config-detail__json-pane">
              <MonacoEditor
                :value="previewValue"
                disabled
              />
            </div>
          </div>
          <div
            v-if="canWriteConfig && tree"
            class="config-detail__content-actions"
          >
            <el-button @click="preview">
              刷新 JSON 预览
            </el-button>
            <el-button
              type="primary"
              :loading="savingValue"
              @click="saveValue"
            >
              保存配置内容
            </el-button>
          </div>
        </el-tab-pane>
        <el-tab-pane
          label="模型定义"
          name="models"
        >
          <ModelsPanel
            v-if="activeTab === 'models'"
            :project-id="projectId"
            :config-id="configId"
            :enums="enums"
            @loaded="models = $event"
            @changed="reload"
          />
        </el-tab-pane>
        <el-tab-pane
          label="枚举定义"
          name="enums"
        >
          <EnumsPanel
            v-if="activeTab === 'enums'"
            :project-id="projectId"
            :config-id="configId"
            @loaded="enums = $event"
            @changed="reload"
          />
        </el-tab-pane>
        <el-tab-pane
          v-if="canReadSnapshots"
          label="快照"
          name="snapshots"
        >
          <SnapshotsPanel
            v-if="activeTab === 'snapshots'"
            :project-id="projectId"
            :config-id="configId"
            @changed="reload"
          />
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog
      :visible.sync="dialog.visible"
      title="修改配置定义"
      width="560px"
      :close-on-click-modal="false"
      append-to-body
    >
      <el-form
        v-if="dialog.visible"
        ref="definitionForm"
        :model="dialog.form"
        :rules="rules"
        label-width="88px"
      >
        <el-form-item label="配置标识">
          <el-input
            :value="config.key"
            disabled
          />
        </el-form-item>
        <el-form-item
          label="配置描述"
          prop="description"
        >
          <el-input
            v-model="dialog.form.description"
            maxlength="255"
            show-word-limit
          />
        </el-form-item>
        <el-form-item
          label="数据类型"
          prop="type"
        >
          <DataTypeSelector
            v-model="dialog.form.type"
            :models="models"
            :enums="enums"
          />
        </el-form-item>
        <el-form-item label="数组">
          <el-switch v-model="dialog.form.isArray" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog.visible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="dialog.saving"
          @click="saveDefinition"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import { getConfig, updateConfig, updateConfigValue } from '@/api/configs'
import { getProject } from '@/api/projects'
import DataTypeSelector from '@/components/DataTypeSelector'
import EnvironmentTag from '@/components/EnvironmentTag'
import MonacoEditor from '@/components/MonacoEditor'
import SchemaForm from '@/components/SchemaForm'
import EnumsPanel from '@/features/config-center/enums/EnumsPanel.vue'
import ModelsPanel from '@/features/config-center/models/ModelsPanel.vue'
import SnapshotsPanel from '@/features/config-center/snapshots/SnapshotsPanel.vue'
import {
  normalizeModel,
  normalizeTree,
  parseEditorType,
  stringifyEditorType,
  toApiType,
  toEditorType,
} from './type-codec'

const FileFieldUnavailable = {
  name: 'FileFieldUnavailable',
  functional: true,
  render(createElement) {
    return createElement(
      'span',
      { class: 'config-detail__file-placeholder' },
      '文件上传将在对象存储接入后启用',
    )
  },
}

function errorMessage(error, fallback) {
  return error?.response?.data?.message || error?.message || fallback
}

function formatJSON(value) {
  if (value === '') {
    return ''
  }
  if (typeof value !== 'string') {
    throw new TypeError('配置值必须是 JSON 字符串')
  }
  return JSON.stringify(JSON.parse(value), null, 2)
}

function requireEnumList(reply) {
  if (!Array.isArray(reply?.list)) {
    throw new TypeError('枚举列表响应缺少 list')
  }
  reply.list.forEach((item) => {
    if (!Array.isArray(item?.values)) {
      throw new TypeError('枚举响应缺少 values')
    }
  })
  return reply.list
}

export default {
  name: 'ConfigDetailPage',

  components: {
    DataTypeSelector,
    EnumsPanel,
    EnvironmentTag,
    ModelsPanel,
    MonacoEditor,
    SchemaForm,
    SnapshotsPanel,
  },

  props: {
    projectId: { type: Number, required: true },
    configId: { type: Number, required: true },
  },

  data() {
    return {
      activeTab: 'content',
      loading: false,
      loadSequence: 0,
      savingValue: false,
      project: null,
      config: null,
      tree: null,
      models: [],
      enums: [],
      previewValue: '',
      fileFieldUnavailable: FileFieldUnavailable,
      dialog: {
        visible: false,
        saving: false,
        form: { description: '', type: '', isArray: false },
      },
      rules: {
        type: [
          { required: true, message: '请选择数据类型', trigger: 'change' },
        ],
      },
    }
  },

  computed: {
    environmentId() {
      return this.$store.state.environment.currentId
    },
    environment() {
      return this.$store.getters['environment/current']
    },
    canWriteConfig() {
      return this.$store.getters['environment/hasPermission']('config:write')
    },
    canReadSnapshots() {
      return this.$store.getters['environment/hasPermission']('snapshot:read')
    },
    typeLabel() {
      if (!this.config?.type) {
        return '未设置'
      }
      const type = toEditorType(this.config.type)
      return type.baseType || type.structureKey || type.enumKey
    },
  },

  watch: {
    environmentId(environmentId, previousId) {
      if (!environmentId) {
        return
      }
      if (
        previousId !== null &&
        previousId !== undefined &&
        String(environmentId) !== String(previousId)
      ) {
        this.project = null
        this.config = null
        this.tree = null
        this.models = []
        this.enums = []
        return
      }
      this.reload()
    },
    projectId() {
      this.reload()
    },
    configId() {
      this.reload()
    },
  },

  created() {
    if (this.environmentId) {
      this.reload()
    }
  },

  methods: {
    async reload() {
      if (!this.environmentId) {
        throw new Error('当前没有可用环境')
      }
      if (!Number.isInteger(this.projectId) || this.projectId <= 0) {
        throw new TypeError('projectId 必须是正整数')
      }
      if (!Number.isInteger(this.configId) || this.configId <= 0) {
        throw new TypeError('configId 必须是正整数')
      }
      const sequence = ++this.loadSequence
      this.loading = true
      try {
        const [projectReply, configReply, modelReply, enumReply] =
          await Promise.all([
            getProject(this.environmentId, this.projectId),
            getConfig(this.environmentId, this.configId),
            this.$store.dispatch('configCenter/loadModels', {
              environmentId: this.environmentId,
              projectId: this.projectId,
              configId: this.configId,
              force: true,
            }),
            this.$store.dispatch('configCenter/loadEnums', {
              environmentId: this.environmentId,
              projectId: this.projectId,
              configId: this.configId,
              force: true,
            }),
          ])
        if (!projectReply?.project) {
          throw new TypeError('项目详情响应缺少 project')
        }
        if (!configReply?.config) {
          throw new TypeError('配置详情响应缺少 config')
        }
        const models = modelReply.list.map(normalizeModel)
        const enums = requireEnumList(enumReply)
        const tree = configReply.tree
          ? normalizeTree(configReply.tree)
          : null
        const previewValue = formatJSON(configReply.config.value || '')

        if (sequence === this.loadSequence) {
          this.project = projectReply.project
          this.config = configReply.config
          this.models = models
          this.enums = enums
          this.tree = tree
          this.previewValue = previewValue
        }
      } catch (error) {
        this.$message.error(errorMessage(error, '加载配置详情失败'))
      } finally {
        if (sequence === this.loadSequence) {
          this.loading = false
        }
      }
    },
    goBack() {
      this.$router.push({
        name: 'project-configs',
        params: { projectId: String(this.projectId) },
      })
    },
    preview() {
      if (!this.$refs.schemaForm) {
        throw new Error('配置表单尚未加载')
      }
      this.previewValue = JSON.stringify(
        this.$refs.schemaForm.getValue(),
        null,
        2,
      )
    },
    async saveValue() {
      this.preview()
      this.savingValue = true
      try {
        await updateConfigValue(this.environmentId, {
          id: this.config.id,
          version: this.config.version,
          value: JSON.stringify(this.$refs.schemaForm.getValue()),
        })
        this.$message.success('配置内容已保存')
        await this.reload()
      } catch (error) {
        this.$message.error(errorMessage(error, '保存配置内容失败'))
      } finally {
        this.savingValue = false
      }
    },
    openConfigEditor() {
      this.dialog.form = {
        description: this.config.description || '',
        type: stringifyEditorType(this.config.type),
        isArray: Boolean(this.config.isArray),
      }
      this.dialog.visible = true
    },
    async saveDefinition() {
      const valid = await new Promise((resolve) => {
        this.$refs.definitionForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      this.dialog.saving = true
      try {
        await updateConfig(this.environmentId, {
          id: this.config.id,
          description: this.dialog.form.description,
          type: toApiType(parseEditorType(this.dialog.form.type)),
          is_array: Boolean(this.dialog.form.isArray),
          version: this.config.version,
        })
        this.dialog.visible = false
        this.$message.success('配置定义已更新')
        await this.reload()
      } catch (error) {
        this.$message.error(errorMessage(error, '保存配置定义失败'))
      } finally {
        this.dialog.saving = false
      }
    },
  },
}
</script>

<style scoped>
.config-detail {
  display: grid;
  gap: 20px;
  min-height: 320px;
}

.config-detail__title,
.config-detail__summary-header,
.config-detail__content-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.config-detail__title {
  justify-content: flex-start;
  font-weight: 600;
}

.config-detail__summary-header {
  margin-bottom: 20px;
}

.config-detail__summary-header h1,
.config-detail__summary-header p {
  margin: 0;
}

.config-detail__summary-header p {
  margin-top: 6px;
  color: #6b7280;
}

.config-detail__tabs {
  min-height: 480px;
}

.config-detail__editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 3fr) minmax(320px, 2fr);
  gap: 20px;
  min-height: 360px;
  padding: 12px 0;
}

.config-detail__form-pane,
.config-detail__json-pane {
  min-width: 0;
  padding: 16px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.config-detail__content-actions {
  justify-content: center;
  padding-top: 16px;
}

@media (max-width: 960px) {
  .config-detail__editor-grid {
    grid-template-columns: 1fr;
  }
}
</style>
