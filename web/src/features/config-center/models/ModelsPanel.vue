<template>
  <section aria-label="模型定义">
    <header class="models-panel__toolbar">
      <p>模型可以被当前配置的字段引用。</p>
      <el-button
        v-if="canWrite"
        type="primary"
        size="small"
        @click="openCreate"
      >
        创建模型
      </el-button>
    </header>

    <el-table
      v-loading="loading"
      :data="models"
      stripe
      empty-text="当前配置还没有模型"
    >
      <el-table-column
        prop="key"
        label="模型标识"
        min-width="150"
      />
      <el-table-column
        prop="name"
        label="名称"
        min-width="150"
      />
      <el-table-column
        prop="description"
        label="描述"
        min-width="220"
      />
      <el-table-column
        label="字段数"
        width="90"
        align="center"
      >
        <template #default="{ row }">
          {{ row.fields.length }}
        </template>
      </el-table-column>
      <el-table-column
        prop="updatedAt"
        label="更新时间"
        width="190"
      />
      <el-table-column
        v-if="canWrite"
        label="操作"
        width="150"
        align="right"
      >
        <template #default="{ row }">
          <el-button
            type="text"
            @click="openEdit(row)"
          >
            编辑
          </el-button>
          <el-button
            class="models-panel__danger"
            type="text"
            @click="remove(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      :visible.sync="dialog.visible"
      :title="dialog.editing ? '编辑模型' : '创建模型'"
      width="1100px"
      :close-on-click-modal="false"
      append-to-body
    >
      <el-form
        v-if="dialog.visible"
        ref="modelForm"
        :model="dialog.form"
        :rules="rules"
        label-width="88px"
      >
        <el-form-item
          label="模型标识"
          prop="key"
        >
          <el-input
            v-model.trim="dialog.form.key"
            :disabled="dialog.editing"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item
          label="模型名称"
          prop="name"
        >
          <el-input
            v-model.trim="dialog.form.name"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item
          label="模型描述"
          prop="description"
        >
          <el-input
            v-model="dialog.form.description"
            type="textarea"
            :rows="2"
            maxlength="255"
            show-word-limit
          />
        </el-form-item>

        <el-form-item
          v-if="dialog.editing"
          label="字段"
        >
          <div class="models-panel__field-header">
            <el-button
              size="small"
              type="primary"
              @click="addField"
            >
              添加字段
            </el-button>
          </div>
          <el-table
            :data="dialog.form.fields"
            border
            size="small"
          >
            <el-table-column
              label="排序"
              width="90"
              align="center"
            >
              <template #default="{ $index }">
                <el-button
                  icon="el-icon-top"
                  type="text"
                  :disabled="$index === 0"
                  aria-label="上移字段"
                  @click="moveField($index, -1)"
                />
                <el-button
                  icon="el-icon-bottom"
                  type="text"
                  :disabled="$index === dialog.form.fields.length - 1"
                  aria-label="下移字段"
                  @click="moveField($index, 1)"
                />
              </template>
            </el-table-column>
            <el-table-column
              label="字段标识"
              min-width="140"
            >
              <template #default="{ row }">
                <el-input
                  v-model.trim="row.key"
                  size="small"
                />
              </template>
            </el-table-column>
            <el-table-column
              label="字段名称"
              min-width="130"
            >
              <template #default="{ row }">
                <el-input
                  v-model.trim="row.name"
                  size="small"
                />
              </template>
            </el-table-column>
            <el-table-column
              label="字段类型"
              min-width="180"
            >
              <template #default="{ row }">
                <DataTypeSelector
                  v-model="row.type"
                  :models="models"
                  :limited-models="limitedModels"
                  :enums="enums"
                  limit
                />
              </template>
            </el-table-column>
            <el-table-column
              label="描述"
              min-width="160"
            >
              <template #default="{ row }">
                <el-input
                  v-model="row.description"
                  size="small"
                />
              </template>
            </el-table-column>
            <el-table-column
              label="数组"
              width="70"
              align="center"
            >
              <template #default="{ row }">
                <el-checkbox v-model="row.isArray" />
              </template>
            </el-table-column>
            <el-table-column
              label="操作"
              width="70"
              align="center"
            >
              <template #default="{ $index }">
                <el-button
                  class="models-panel__danger"
                  type="text"
                  @click="removeField($index)"
                >
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
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
import { createModel, deleteModel, updateModel } from '@/api/models'
import DataTypeSelector from '@/components/DataTypeSelector'
import {
  normalizeModel,
  parseEditorType,
  toApiType,
} from '@/features/config-center/configs/type-codec'

function errorMessage(error, fallback) {
  return error?.response?.data?.message || error?.message || fallback
}

function emptyModel() {
  return { key: '', name: '', description: '', fields: [] }
}

function requireFields(fields) {
  if (!Array.isArray(fields) || fields.length === 0) {
    throw new Error('模型至少需要一个字段')
  }
  const keys = new Set()
  fields.forEach((field, index) => {
    if (!/^[A-Za-z][A-Za-z0-9]*$/.test(field.key)) {
      throw new Error(`第 ${index + 1} 个字段的标识不合法`)
    }
    if (!field.name) {
      throw new Error(`第 ${index + 1} 个字段缺少名称`)
    }
    if (keys.has(field.key)) {
      throw new Error(`字段标识重复: ${field.key}`)
    }
    keys.add(field.key)
    parseEditorType(field.type)
  })
}

export default {
  name: 'ModelsPanel',

  components: { DataTypeSelector },

  props: {
    projectId: { type: Number, required: true },
    configId: { type: Number, required: true },
    enums: { type: Array, default: () => [] },
  },

  data() {
    return {
      loading: false,
      models: [],
      dialog: {
        visible: false,
        editing: false,
        saving: false,
        form: emptyModel(),
      },
      rules: {
        key: [
          { required: true, message: '请输入模型标识', trigger: 'blur' },
          {
            pattern: /^[A-Za-z][A-Za-z0-9]*$/,
            message: '模型标识只能包含字母和数字，且以字母开头',
            trigger: 'blur',
          },
        ],
        name: [
          { required: true, message: '请输入模型名称', trigger: 'blur' },
        ],
      },
    }
  },

  computed: {
    environmentId() {
      return this.$store.state.environment.currentId
    },
    canWrite() {
      return this.$store.getters['environment/hasPermission'](
        'structure:write',
      )
    },
    limitedModels() {
      if (!this.dialog.editing) {
        return this.models
      }
      return this.models.filter((item) => item.id !== this.dialog.form.id)
    },
  },

  watch: {
    configId() {
      this.reload()
    },
  },

  created() {
    this.reload()
  },

  methods: {
    async reload(force = false) {
      this.loading = true
      try {
        const reply = await this.$store.dispatch('configCenter/loadModels', {
          environmentId: this.environmentId,
          projectId: this.projectId,
          configId: this.configId,
          force,
        })
        this.models = reply.list.map(normalizeModel)
        this.$emit('loaded', this.models)
      } catch (error) {
        this.$message.error(errorMessage(error, '加载模型失败'))
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      this.dialog.editing = false
      this.dialog.form = emptyModel()
      this.dialog.visible = true
    },
    openEdit(model) {
      this.dialog.editing = true
      this.dialog.form = {
        id: model.id,
        key: model.key,
        name: model.name,
        description: model.description || '',
        version: model.version,
        fields: model.fields.map((field) => ({
          ...field,
          type: JSON.stringify(field.type),
        })),
      }
      this.dialog.visible = true
    },
    addField() {
      this.dialog.form.fields.push({
        key: '',
        name: '',
        description: '',
        isArray: false,
        type: JSON.stringify({ baseType: 'String' }),
      })
    },
    removeField(index) {
      this.dialog.form.fields.splice(index, 1)
    },
    moveField(index, offset) {
      const target = index + offset
      if (target < 0 || target >= this.dialog.form.fields.length) {
        return
      }
      const fields = [...this.dialog.form.fields]
      const current = fields[index]
      fields[index] = fields[target]
      fields[target] = current
      this.dialog.form.fields = fields
    },
    async save() {
      const valid = await new Promise((resolve) => {
        this.$refs.modelForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      this.dialog.saving = true
      try {
        if (this.dialog.editing) {
          requireFields(this.dialog.form.fields)
          await updateModel(this.environmentId, {
            id: this.dialog.form.id,
            key: this.dialog.form.key,
            name: this.dialog.form.name,
            description: this.dialog.form.description,
            version: this.dialog.form.version,
            fields: this.dialog.form.fields.map((field) => ({
              key: field.key,
              name: field.name,
              description: field.description || '',
              is_array: Boolean(field.isArray),
              type: toApiType(parseEditorType(field.type)),
            })),
          })
        } else {
          await createModel(this.environmentId, {
            config_id: this.configId,
            key: this.dialog.form.key,
            name: this.dialog.form.name,
            description: this.dialog.form.description,
          })
        }
        this.dialog.visible = false
        this.$message.success(this.dialog.editing ? '模型已更新' : '模型已创建')
        await this.reload(true)
        this.$emit('changed')
      } catch (error) {
        this.$message.error(errorMessage(error, '保存模型失败'))
      } finally {
        this.dialog.saving = false
      }
    },
    async remove(model) {
      try {
        await this.$confirm(`确认删除模型“${model.name}”吗？`, '删除模型', {
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
        await deleteModel(this.environmentId, model.id)
        this.$message.success('模型已删除')
        await this.reload(true)
        this.$emit('changed')
      } catch (error) {
        this.$message.error(errorMessage(error, '删除模型失败'))
      }
    },
  },
}
</script>

<style scoped>
.models-panel__toolbar,
.models-panel__field-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.models-panel__toolbar p {
  margin: 0;
  color: #6b7280;
}

.models-panel__field-header {
  justify-content: flex-end;
}

.models-panel__danger {
  color: #f56c6c;
}
</style>
