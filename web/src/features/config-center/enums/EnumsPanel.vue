<template>
  <section aria-label="枚举定义">
    <header class="enums-panel__toolbar">
      <p>枚举值用于限制配置字段的可选范围。</p>
      <el-button
        v-if="canWrite"
        type="primary"
        size="small"
        @click="openCreate"
      >
        创建枚举
      </el-button>
    </header>

    <el-table
      v-loading="loading"
      :data="enums"
      stripe
      empty-text="当前配置还没有枚举"
    >
      <el-table-column
        prop="key"
        label="枚举标识"
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
        label="枚举值数"
        width="100"
        align="center"
      >
        <template #default="{ row }">
          {{ row.values.length }}
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
            class="enums-panel__danger"
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
      :title="dialog.editing ? '编辑枚举' : '创建枚举'"
      width="900px"
      :close-on-click-modal="false"
      append-to-body
    >
      <el-form
        v-if="dialog.visible"
        ref="enumForm"
        :model="dialog.form"
        :rules="rules"
        label-width="88px"
      >
        <el-form-item
          label="枚举标识"
          prop="key"
        >
          <el-input
            v-model.trim="dialog.form.key"
            :disabled="dialog.editing"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item
          label="枚举名称"
          prop="name"
        >
          <el-input
            v-model.trim="dialog.form.name"
            maxlength="64"
          />
        </el-form-item>
        <el-form-item
          label="枚举描述"
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
        <el-form-item label="枚举值">
          <div class="enums-panel__value-header">
            <el-button
              type="primary"
              size="small"
              @click="addValue"
            >
              添加枚举值
            </el-button>
          </div>
          <el-table
            :data="dialog.form.values"
            border
            size="small"
          >
            <el-table-column
              label="显示文本"
              min-width="180"
            >
              <template #default="{ row }">
                <el-input
                  v-model.trim="row.label"
                  size="small"
                />
              </template>
            </el-table-column>
            <el-table-column
              label="枚举值"
              min-width="180"
            >
              <template #default="{ row }">
                <el-input
                  v-model.trim="row.value"
                  size="small"
                  placeholder="例如 ENABLED"
                />
              </template>
            </el-table-column>
            <el-table-column
              label="描述"
              min-width="220"
            >
              <template #default="{ row }">
                <el-input
                  v-model="row.description"
                  size="small"
                />
              </template>
            </el-table-column>
            <el-table-column
              label="操作"
              width="70"
              align="center"
            >
              <template #default="{ $index }">
                <el-button
                  class="enums-panel__danger"
                  type="text"
                  @click="removeValue($index)"
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
import { createEnum, deleteEnum, updateEnum } from '@/api/enums'

function errorMessage(error, fallback) {
  return error?.response?.data?.message || error?.message || fallback
}

function emptyEnum() {
  return {
    key: '',
    name: '',
    description: '',
    values: [{ label: '', value: '', description: '' }],
  }
}

function requireValues(values) {
  if (!Array.isArray(values) || values.length === 0) {
    throw new Error('枚举至少需要一个值')
  }
  const keys = new Set()
  values.forEach((item, index) => {
    if (!item.label) {
      throw new Error(`第 ${index + 1} 个枚举值缺少显示文本`)
    }
    if (!/^[A-Z][A-Z0-9_]*$/.test(item.value)) {
      throw new Error(`第 ${index + 1} 个枚举值必须使用大写字母、数字或下划线`)
    }
    if (keys.has(item.value)) {
      throw new Error(`枚举值重复: ${item.value}`)
    }
    keys.add(item.value)
  })
}

export default {
  name: 'EnumsPanel',

  props: {
    projectId: { type: Number, required: true },
    configId: { type: Number, required: true },
  },

  data() {
    return {
      loading: false,
      enums: [],
      dialog: {
        visible: false,
        editing: false,
        saving: false,
        form: emptyEnum(),
      },
      rules: {
        key: [
          { required: true, message: '请输入枚举标识', trigger: 'blur' },
          {
            pattern: /^[A-Za-z][A-Za-z0-9]*$/,
            message: '枚举标识只能包含字母和数字，且以字母开头',
            trigger: 'blur',
          },
        ],
        name: [
          { required: true, message: '请输入枚举名称', trigger: 'blur' },
        ],
      },
    }
  },

  computed: {
    environmentId() {
      return this.$store.state.environment.currentId
    },
    canWrite() {
      return this.$store.getters['environment/hasPermission']('enum:write')
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
        const reply = await this.$store.dispatch('configCenter/loadEnums', {
          environmentId: this.environmentId,
          projectId: this.projectId,
          configId: this.configId,
          force,
        })
        this.enums = reply.list.map((item) => {
          if (!Array.isArray(item?.values)) {
            throw new TypeError('枚举响应缺少 values')
          }
          return item
        })
        this.$emit('loaded', this.enums)
      } catch (error) {
        this.$message.error(errorMessage(error, '加载枚举失败'))
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      this.dialog.editing = false
      this.dialog.form = emptyEnum()
      this.dialog.visible = true
    },
    openEdit(configEnum) {
      this.dialog.editing = true
      this.dialog.form = {
        id: configEnum.id,
        key: configEnum.key,
        name: configEnum.name,
        description: configEnum.description || '',
        values: configEnum.values.map((item) => ({ ...item })),
        version: configEnum.version,
      }
      this.dialog.visible = true
    },
    addValue() {
      this.dialog.form.values.push({
        label: '',
        value: '',
        description: '',
      })
    },
    removeValue(index) {
      this.dialog.form.values.splice(index, 1)
    },
    async save() {
      const valid = await new Promise((resolve) => {
        this.$refs.enumForm.validate(resolve)
      })
      if (!valid) {
        return
      }
      this.dialog.saving = true
      try {
        requireValues(this.dialog.form.values)
        const request = {
          key: this.dialog.form.key,
          name: this.dialog.form.name,
          description: this.dialog.form.description,
          values: this.dialog.form.values,
        }
        if (this.dialog.editing) {
          await updateEnum(this.environmentId, {
            ...request,
            id: this.dialog.form.id,
            version: this.dialog.form.version,
          })
        } else {
          await createEnum(this.environmentId, {
            ...request,
            config_id: this.configId,
          })
        }
        this.dialog.visible = false
        this.$message.success(this.dialog.editing ? '枚举已更新' : '枚举已创建')
        await this.reload(true)
        this.$emit('changed')
      } catch (error) {
        this.$message.error(errorMessage(error, '保存枚举失败'))
      } finally {
        this.dialog.saving = false
      }
    },
    async remove(configEnum) {
      try {
        await this.$confirm(`确认删除枚举“${configEnum.name}”吗？`, '删除枚举', {
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
        await deleteEnum(this.environmentId, configEnum.id)
        this.$message.success('枚举已删除')
        await this.reload(true)
        this.$emit('changed')
      } catch (error) {
        this.$message.error(errorMessage(error, '删除枚举失败'))
      }
    },
  },
}
</script>

<style scoped>
.enums-panel__toolbar,
.enums-panel__value-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.enums-panel__toolbar p {
  margin: 0;
  color: #6b7280;
}

.enums-panel__value-header {
  justify-content: flex-end;
}

.enums-panel__danger {
  color: #f56c6c;
}
</style>
