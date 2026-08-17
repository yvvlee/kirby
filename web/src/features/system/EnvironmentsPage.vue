<template>
  <section class="management-card">
    <header class="management-card__header">
      <div>
        <h2>环境</h2>
        <p>环境是项目、成员角色和配置数据的隔离边界。</p>
      </div>
      <el-button
        v-if="systemAdmin"
        type="primary"
        @click="openCreate"
      >
        新建环境
      </el-button>
    </header>

    <el-alert
      v-if="errorMessage"
      class="management-card__alert"
      :title="errorMessage"
      type="error"
      :closable="false"
      show-icon
    />

    <el-alert
      v-if="!systemAdmin"
      title="只有系统管理员可以修改环境"
      type="warning"
      :closable="false"
      show-icon
    />

    <el-table
      v-else
      v-loading="loading"
      :data="environments"
      empty-text="暂无环境"
    >
      <el-table-column
        prop="name"
        label="名称"
        min-width="160"
      />
      <el-table-column
        prop="key"
        label="标识"
        min-width="140"
      />
      <el-table-column
        prop="description"
        label="说明"
        min-width="220"
      />
      <el-table-column
        label="状态"
        width="100"
      >
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">
            {{ row.enabled ? '启用' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column
        label="操作"
        width="100"
        fixed="right"
      >
        <template #default="{ row }">
          <el-button
            type="text"
            @click="openEdit(row)"
          >
            编辑
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      :title="editing ? '编辑环境' : '新建环境'"
      :visible.sync="dialogVisible"
      width="520px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="environmentForm"
        :model="form"
        :rules="rules"
        label-position="top"
      >
        <el-form-item
          label="环境标识"
          prop="key"
        >
          <el-input
            v-model.trim="form.key"
            :disabled="editing"
            placeholder="例如 production"
          />
        </el-form-item>
        <el-form-item
          label="名称"
          prop="name"
        >
          <el-input v-model.trim="form.name" />
        </el-form-item>
        <el-form-item
          label="说明"
          prop="description"
        >
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
          />
        </el-form-item>
        <el-form-item
          v-if="editing"
          label="状态"
        >
          <el-switch
            v-model="form.enabled"
            active-text="启用"
            inactive-text="停用"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="save"
        >
          保存
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import {
  createEnvironment,
  updateEnvironment,
} from '@/api/environments'

import { actionErrorMessage } from './errors'

function emptyForm() {
  return {
    id: null,
    key: '',
    name: '',
    description: '',
    enabled: true,
    version: 0,
  }
}

export default {
  name: 'EnvironmentsPage',

  data() {
    return {
      loading: false,
      saving: false,
      dialogVisible: false,
      errorMessage: '',
      form: emptyForm(),
      rules: {
        key: [
          { required: true, message: '请输入环境标识', trigger: 'blur' },
          {
            pattern: /^[a-z][a-z0-9-]*$/,
            message: '只能使用小写字母、数字和连字符，且必须以字母开头',
            trigger: 'blur',
          },
        ],
        name: [{ required: true, message: '请输入环境名称', trigger: 'blur' }],
        description: [
          { max: 255, message: '说明不能超过 255 个字符', trigger: 'blur' },
        ],
      },
    }
  },

  computed: {
    systemAdmin() {
      return this.$store.getters['session/systemAdmin']
    },
    environments() {
      return this.$store.state.environment.available
    },
    editing() {
      return this.form.id !== null
    },
  },

  created() {
    if (this.systemAdmin) {
      this.load()
    }
  },

  methods: {
    async load() {
      this.loading = true
      this.errorMessage = ''
      try {
        await this.$store.dispatch('environment/loadAvailable')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '读取环境')
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      this.form = emptyForm()
      this.errorMessage = ''
      this.dialogVisible = true
      this.$nextTick(() => this.$refs.environmentForm?.clearValidate())
    },
    openEdit(environment) {
      this.form = {
        id: environment.id,
        key: environment.key,
        name: environment.name,
        description: environment.description || '',
        enabled: environment.enabled,
        version: environment.version,
      }
      this.errorMessage = ''
      this.dialogVisible = true
      this.$nextTick(() => this.$refs.environmentForm?.clearValidate())
    },
    async save() {
      await this.$refs.environmentForm.validate()
      this.saving = true
      this.errorMessage = ''
      try {
        if (this.editing) {
          await updateEnvironment(this.form.id, {
            name: this.form.name,
            description: this.form.description,
            enabled: this.form.enabled,
            version: this.form.version,
          })
        } else {
          await createEnvironment({
            key: this.form.key,
            name: this.form.name,
            description: this.form.description,
          })
        }
        this.dialogVisible = false
        await this.load()
        this.$message.success('环境已保存')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '保存环境')
      } finally {
        this.saving = false
      }
    },
  },
}
</script>

<style scoped src="./management-card.css"></style>
