<template>
  <section class="management-card">
    <header class="management-card__header">
      <div>
        <h2>系统用户</h2>
        <p>用户状态和系统管理员标记是全局设置。</p>
      </div>
      <el-button
        v-if="systemAdmin"
        type="primary"
        @click="openCreate"
      >
        新建用户
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
      title="环境管理员不能修改全局用户状态"
      type="warning"
      :closable="false"
      show-icon
    />

    <el-table
      v-else
      v-loading="loading"
      :data="users"
      empty-text="暂无用户"
    >
      <el-table-column
        prop="id"
        label="ID"
        width="90"
      />
      <el-table-column
        prop="username"
        label="用户名"
        min-width="150"
      />
      <el-table-column
        prop="display_name"
        label="显示名称"
        min-width="150"
      />
      <el-table-column
        label="身份"
        width="130"
      >
        <template #default="{ row }">
          <el-tag v-if="row.is_system_admin">
            系统管理员
          </el-tag>
          <span v-else>普通用户</span>
        </template>
      </el-table-column>
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
            @click="openPassword(row)"
          >
            改密码
          </el-button>
          <el-button
            type="text"
            @click="toggleStatus(row)"
          >
            {{ row.enabled ? '停用' : '启用' }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      :title="editing ? '编辑用户' : '新建用户'"
      :visible.sync="userDialogVisible"
      width="520px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="userForm"
        :model="form"
        :rules="userRules"
        label-position="top"
      >
        <el-form-item
          label="用户名"
          prop="username"
        >
          <el-input
            v-model.trim="form.username"
            :disabled="editing"
            autocomplete="off"
          />
        </el-form-item>
        <el-form-item
          label="显示名称"
          prop="display_name"
        >
          <el-input v-model.trim="form.display_name" />
        </el-form-item>
        <el-form-item
          v-if="!editing"
          label="初始密码"
          prop="password"
        >
          <el-input
            v-model="form.password"
            type="password"
            show-password
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="系统管理员">
          <el-switch v-model="form.is_system_admin" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="closeUserDialog">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          @click="saveUser"
        >
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      title="修改密码"
      :visible.sync="passwordDialogVisible"
      width="460px"
      :close-on-click-modal="false"
      @closed="clearPassword"
    >
      <el-form
        ref="passwordForm"
        :model="passwordForm"
        :rules="passwordRules"
        label-position="top"
      >
        <el-form-item
          label="新密码"
          prop="password"
        >
          <el-input
            v-model="passwordForm.password"
            type="password"
            show-password
            autocomplete="new-password"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="savingPassword"
          @click="savePassword"
        >
          保存密码
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import {
  createUser,
  listUsers,
  updateUser,
  updateUserPassword,
  updateUserStatus,
} from '@/api/users'

import { actionErrorMessage, requireList } from './errors'

function emptyUserForm() {
  return {
    id: null,
    username: '',
    display_name: '',
    password: '',
    is_system_admin: false,
    version: 0,
  }
}

export default {
  name: 'UsersPage',

  data() {
    return {
      users: [],
      loading: false,
      saving: false,
      savingPassword: false,
      errorMessage: '',
      userDialogVisible: false,
      passwordDialogVisible: false,
      form: emptyUserForm(),
      passwordForm: { userId: null, password: '' },
      userRules: {
        username: [
          { required: true, message: '请输入用户名', trigger: 'blur' },
          { max: 128, message: '用户名不能超过 128 个字符', trigger: 'blur' },
        ],
        display_name: [
          { required: true, message: '请输入显示名称', trigger: 'blur' },
          { max: 128, message: '显示名称不能超过 128 个字符', trigger: 'blur' },
        ],
        password: [
          { required: true, message: '请输入初始密码', trigger: 'blur' },
          { min: 12, message: '密码至少需要 12 个字符', trigger: 'blur' },
        ],
      },
      passwordRules: {
        password: [
          { required: true, message: '请输入新密码', trigger: 'blur' },
          { min: 12, message: '密码至少需要 12 个字符', trigger: 'blur' },
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
        this.users = requireList(await listUsers(), 'user list')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '读取用户')
      } finally {
        this.loading = false
      }
    },
    openCreate() {
      this.form = emptyUserForm()
      this.errorMessage = ''
      this.userDialogVisible = true
      this.$nextTick(() => this.$refs.userForm?.clearValidate())
    },
    openEdit(user) {
      this.form = {
        id: user.id,
        username: user.username,
        display_name: user.display_name,
        password: '',
        is_system_admin: user.is_system_admin,
        version: user.version,
      }
      this.errorMessage = ''
      this.userDialogVisible = true
      this.$nextTick(() => this.$refs.userForm?.clearValidate())
    },
    closeUserDialog() {
      this.userDialogVisible = false
      this.form.password = ''
    },
    async saveUser() {
      await this.$refs.userForm.validate()
      this.saving = true
      this.errorMessage = ''
      try {
        if (this.editing) {
          await updateUser(this.form.id, {
            display_name: this.form.display_name,
            is_system_admin: this.form.is_system_admin,
            version: this.form.version,
          })
        } else {
          await createUser({
            username: this.form.username,
            display_name: this.form.display_name,
            password: this.form.password,
            is_system_admin: this.form.is_system_admin,
          })
        }
        this.closeUserDialog()
        await this.load()
        this.$message.success('用户已保存')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '保存用户')
      } finally {
        this.form.password = ''
        this.saving = false
      }
    },
    openPassword(user) {
      this.passwordForm = { userId: user.id, password: '' }
      this.errorMessage = ''
      this.passwordDialogVisible = true
      this.$nextTick(() => this.$refs.passwordForm?.clearValidate())
    },
    clearPassword() {
      this.passwordForm = { userId: null, password: '' }
    },
    async savePassword() {
      await this.$refs.passwordForm.validate()
      this.savingPassword = true
      this.errorMessage = ''
      try {
        await updateUserPassword(
          this.passwordForm.userId,
          this.passwordForm.password,
        )
        this.passwordDialogVisible = false
        this.$message.success('密码已更新')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '修改密码')
      } finally {
        this.passwordForm.password = ''
        this.savingPassword = false
      }
    },
    async toggleStatus(user) {
      try {
        await this.$confirm(
          `确认${user.enabled ? '停用' : '启用'}用户“${
            user.display_name || user.username
          }”吗？`,
          '确认用户状态',
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
        await updateUserStatus(user.id, !user.enabled, user.version)
        await this.load()
        this.$message.success('用户状态已更新')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '修改用户状态')
      }
    },
  },
}
</script>

<style scoped src="./management-card.css"></style>
