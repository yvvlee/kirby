<template>
  <section class="management-card">
    <header class="management-card__header">
      <div>
        <h2>角色与权限</h2>
        <p>权限清单由后端返回。页面不会补充未声明权限。</p>
      </div>
      <el-button
        v-if="systemAdmin"
        type="primary"
        @click="openCreate"
      >
        新建角色
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
      title="只有系统管理员可以修改全局角色与权限"
      type="warning"
      :closable="false"
      show-icon
    />

    <el-table
      v-else
      v-loading="loading"
      :data="roles"
      empty-text="暂无角色"
    >
      <el-table-column
        prop="name"
        label="名称"
        min-width="140"
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
        label="权限数"
        width="90"
      >
        <template #default="{ row }">
          {{ Array.isArray(row.permissions) ? row.permissions.length : 0 }}
        </template>
      </el-table-column>
      <el-table-column
        label="类型"
        width="100"
      >
        <template #default="{ row }">
          <el-tag :type="row.builtin ? 'info' : ''">
            {{ row.builtin ? '内置' : '自定义' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column
        label="操作"
        width="250"
        fixed="right"
      >
        <template #default="{ row }">
          <el-button
            type="text"
            @click="openEdit(row)"
          >
            编辑
          </el-button>
          <el-button
            type="text"
            @click="openPermissions(row)"
          >
            配置权限
          </el-button>
          <el-button
            v-if="!row.builtin"
            type="text"
            class="danger-action"
            @click="removeRole(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      :title="editing ? '编辑角色' : '新建角色'"
      :visible.sync="roleDialogVisible"
      width="520px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="roleForm"
        :model="form"
        :rules="rules"
        label-position="top"
      >
        <el-form-item
          label="角色标识"
          prop="key"
        >
          <el-input
            v-model.trim="form.key"
            :disabled="editing"
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
      </el-form>
      <template #footer>
        <el-button @click="roleDialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="saveRole"
        >
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      :title="permissionRole ? `配置“${permissionRole.name}”的权限` : '配置权限'"
      :visible.sync="permissionDialogVisible"
      width="660px"
      :close-on-click-modal="false"
    >
      <el-alert
        v-if="permissions.length === 0"
        title="后端没有返回可分配权限"
        type="warning"
        :closable="false"
        show-icon
      />
      <el-checkbox-group
        v-else
        v-model="selectedPermissionIds"
        class="permission-list"
        aria-label="权限清单"
      >
        <div
          v-for="permission in permissions"
          :key="String(permission.id)"
          class="permission-list__item"
        >
          <el-checkbox :label="permission.id">
            {{ permission.name }}（{{ permission.key }}）
          </el-checkbox>
          <p class="permission-list__description">
            {{ permission.description || '无说明' }}
          </p>
        </div>
      </el-checkbox-group>
      <template #footer>
        <el-button @click="permissionDialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="savingPermissions"
          :disabled="permissions.length === 0"
          @click="savePermissions"
        >
          保存权限
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import {
  createRole,
  deleteRole,
  listPermissions,
  listRoles,
  updateRole,
  updateRolePermissions,
} from '@/api/roles'

import { actionErrorMessage, requireList } from './errors'

function emptyRoleForm() {
  return {
    id: null,
    key: '',
    name: '',
    description: '',
    version: 0,
  }
}

export default {
  name: 'RolesPage',

  data() {
    return {
      roles: [],
      permissions: [],
      selectedPermissionIds: [],
      permissionRole: null,
      loading: false,
      saving: false,
      savingPermissions: false,
      errorMessage: '',
      roleDialogVisible: false,
      permissionDialogVisible: false,
      form: emptyRoleForm(),
      rules: {
        key: [
          { required: true, message: '请输入角色标识', trigger: 'blur' },
          {
            pattern: /^[a-z][a-z0-9_:.-]*$/,
            message: '角色标识格式不正确',
            trigger: 'blur',
          },
        ],
        name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
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
        const [roleReply, permissionReply] = await Promise.all([
          listRoles(),
          listPermissions(),
        ])
        this.roles = requireList(roleReply, 'role list')
        this.permissions = requireList(permissionReply, 'permission list')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '读取角色与权限')
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      this.form = emptyRoleForm()
      this.errorMessage = ''
      this.roleDialogVisible = true
      this.$nextTick(() => this.$refs.roleForm?.clearValidate())
    },
    openEdit(role) {
      this.form = {
        id: role.id,
        key: role.key,
        name: role.name,
        description: role.description || '',
        version: role.version,
      }
      this.errorMessage = ''
      this.roleDialogVisible = true
      this.$nextTick(() => this.$refs.roleForm?.clearValidate())
    },
    async saveRole() {
      await this.$refs.roleForm.validate()
      this.saving = true
      this.errorMessage = ''
      try {
        if (this.editing) {
          await updateRole(this.form.id, {
            name: this.form.name,
            description: this.form.description,
            version: this.form.version,
          })
        } else {
          await createRole({
            key: this.form.key,
            name: this.form.name,
            description: this.form.description,
          })
        }
        this.roleDialogVisible = false
        await this.load()
        this.$message.success('角色已保存')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '保存角色')
      } finally {
        this.saving = false
      }
    },
    openPermissions(role) {
      this.permissionRole = role
      this.selectedPermissionIds = Array.isArray(role.permissions)
        ? role.permissions.map((permission) => permission.id)
        : []
      this.errorMessage = ''
      this.permissionDialogVisible = true
    },
    async savePermissions() {
      if (!this.permissionRole) {
        return
      }
      this.savingPermissions = true
      this.errorMessage = ''
      try {
        await updateRolePermissions(
          this.permissionRole.id,
          this.selectedPermissionIds,
        )
        this.permissionDialogVisible = false
        await this.load()
        this.$message.success('角色权限已更新')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '保存角色权限')
      } finally {
        this.savingPermissions = false
      }
    },
    async removeRole(role) {
      try {
        await this.$confirm(
          `确认删除角色“${role.name}”吗？仍被成员使用的角色不能删除。`,
          '确认删除',
          { type: 'warning' },
        )
      } catch (error) {
        if (error === 'cancel' || error === 'close') {
          return
        }
        throw error
      }
      this.errorMessage = ''
      try {
        await deleteRole(role.id)
        await this.load()
        this.$message.success('角色已删除')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '删除角色')
      }
    },
  },
}
</script>

<style scoped src="./management-card.css"></style>

<style scoped>
.danger-action {
  color: #dc2626;
}
</style>
