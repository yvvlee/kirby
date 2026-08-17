<template>
  <section
    class="configs-page"
    aria-labelledby="configs-title"
  >
    <el-page-header @back="goToProjects">
      <template #content>
        <div class="configs-page__title">
          <span id="configs-title">
            {{ project ? project.name : '项目配置' }}
          </span>
          <EnvironmentTag :environment="environment" />
        </div>
      </template>
    </el-page-header>

    <el-card shadow="never">
      <header class="configs-page__toolbar">
        <el-form
          inline
          size="small"
          @submit.native.prevent="reload"
        >
          <el-form-item label="配置标识">
            <el-input
              v-model.trim="keyFilter"
              clearable
              placeholder="输入完整配置标识"
              @clear="reload"
            />
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              :loading="loading"
              @click="reload"
            >
              搜索
            </el-button>
          </el-form-item>
        </el-form>
        <el-button
          v-if="canWrite"
          type="primary"
          size="small"
          @click="openCreate"
        >
          创建配置
        </el-button>
      </header>

      <el-table
        v-loading="loading"
        :data="configs"
        stripe
        empty-text="当前项目还没有配置"
      >
        <el-table-column
          prop="key"
          label="配置标识"
          min-width="180"
        />
        <el-table-column
          prop="description"
          label="描述"
          min-width="240"
        />
        <el-table-column
          label="发布状态"
          width="120"
          align="center"
        >
          <template #default="{ row }">
            <el-tag
              :type="row.isReleased ? 'success' : 'info'"
              size="small"
            >
              {{ row.isReleased ? '已发布' : '未发布' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column
          label="更新时间"
          prop="updatedAt"
          width="190"
        />
        <el-table-column
          label="操作"
          width="190"
          align="right"
        >
          <template #default="{ row }">
            <el-button
              type="text"
              @click="openConfig(row)"
            >
              查看详情
            </el-button>
            <el-button
              v-if="canWrite"
              class="configs-page__danger"
              type="text"
              @click="remove(row)"
            >
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog
      :visible.sync="dialog.visible"
      title="创建配置"
      width="540px"
      :close-on-click-modal="false"
      append-to-body
    >
      <el-form
        v-if="dialog.visible"
        ref="configForm"
        :model="dialog.form"
        :rules="rules"
        label-width="88px"
      >
        <el-form-item
          label="配置标识"
          prop="key"
        >
          <el-input
            v-model.trim="dialog.form.key"
            maxlength="64"
            placeholder="例如 FeatureFlags"
          />
        </el-form-item>
        <el-form-item
          label="配置描述"
          prop="description"
        >
          <el-input
            v-model="dialog.form.description"
            type="textarea"
            :rows="3"
            maxlength="255"
            show-word-limit
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog.visible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="dialog.saving"
          @click="save"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import { createConfig, deleteConfig } from '@/api/configs'
import { getProject } from '@/api/projects'
import EnvironmentTag from '@/components/EnvironmentTag'

function errorMessage(error, fallback) {
  return error?.response?.data?.message || error?.message || fallback
}

function normalizeConfigList(reply) {
  if (!Array.isArray(reply?.list)) {
    throw new TypeError('配置列表响应缺少 list')
  }
  return reply.list.map((item) => {
    if (!item?.config) {
      throw new TypeError('配置列表项缺少 config')
    }
    return { ...item.config, isReleased: Boolean(item.isReleased) }
  })
}

export default {
  name: 'ConfigsPage',

  components: { EnvironmentTag },

  props: {
    projectId: {
      type: Number,
      required: true,
    },
  },

  data() {
    return {
      project: null,
      configs: [],
      keyFilter: '',
      loading: false,
      loadSequence: 0,
      dialog: {
        visible: false,
        saving: false,
        form: { key: '', description: '' },
      },
      rules: {
        key: [
          { required: true, message: '请输入配置标识', trigger: 'blur' },
          {
            pattern: /^[A-Za-z][A-Za-z0-9]*$/,
            message: '配置标识只能包含字母和数字，且以字母开头',
            trigger: 'blur',
          },
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
    canWrite() {
      return this.$store.getters['environment/hasPermission']('config:write')
    },
  },

  watch: {
    environmentId(environmentId, previousId) {
      if (!environmentId) {
        this.configs = []
        return
      }
      if (
        previousId !== null &&
        previousId !== undefined &&
        String(environmentId) !== String(previousId)
      ) {
        this.project = null
        this.configs = []
        return
      }
      this.reload()
    },
    projectId() {
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
      const sequence = ++this.loadSequence
      this.loading = true
      try {
        const [projectReply, configReply] = await Promise.all([
          getProject(this.environmentId, this.projectId),
          this.$store.dispatch('configCenter/loadConfigs', {
            environmentId: this.environmentId,
            projectId: this.projectId,
            filter: this.keyFilter ? { key: this.keyFilter } : {},
            force: true,
          }),
        ])
        if (!projectReply?.project) {
          throw new TypeError('项目详情响应缺少 project')
        }
        if (sequence === this.loadSequence) {
          this.project = projectReply.project
          this.configs = normalizeConfigList(configReply)
        }
      } catch (error) {
        this.$message.error(errorMessage(error, '加载配置失败'))
      } finally {
        if (sequence === this.loadSequence) {
          this.loading = false
        }
      }
    },
    goToProjects(replace = false) {
      const navigation = { name: 'project-list' }
      return replace
        ? this.$router.replace(navigation)
        : this.$router.push(navigation)
    },
    openConfig(config) {
      this.$router.push({
        name: 'config-detail',
        params: {
          projectId: String(this.projectId),
          configId: String(config.id),
        },
      })
    },
    openCreate() {
      this.dialog.form = { key: '', description: '' }
      this.dialog.visible = true
    },
    async save() {
      const valid = await new Promise((resolve) => {
        this.$refs.configForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      this.dialog.saving = true
      try {
        await createConfig(this.environmentId, {
          ...this.dialog.form,
          project_id: this.projectId,
        })
        this.dialog.visible = false
        this.$message.success('配置已创建')
        await this.reload()
      } catch (error) {
        this.$message.error(errorMessage(error, '创建配置失败'))
      } finally {
        this.dialog.saving = false
      }
    },
    async remove(config) {
      try {
        await this.$confirm(
          `确认删除配置“${config.description || config.key}”吗？`,
          '删除配置',
          { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' },
        )
      } catch (error) {
        if (error === 'cancel' || error === 'close') {
          return
        }
        throw error
      }

      try {
        await deleteConfig(this.environmentId, config.id)
        this.$message.success('配置已删除')
        await this.reload()
      } catch (error) {
        this.$message.error(errorMessage(error, '删除配置失败'))
      }
    },
  },
}
</script>

<style scoped>
.configs-page {
  display: grid;
  gap: 20px;
}

.configs-page__title,
.configs-page__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.configs-page__title {
  justify-content: flex-start;
  font-weight: 600;
}

.configs-page__danger {
  color: #f56c6c;
}
</style>
