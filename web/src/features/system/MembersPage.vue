<template>
  <section class="management-card">
    <header class="management-card__header">
      <div>
        <h2>当前环境成员</h2>
        <p v-if="currentEnvironment">
          只修改“{{ currentEnvironment.name }}”中的成员角色。
        </p>
        <p v-else>
          请先选择一个可用环境。
        </p>
      </div>
      <el-button
        v-if="canManage && currentEnvironment"
        type="primary"
        @click="openAdd"
      >
        添加成员
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
      v-if="!canManage"
      title="当前账号没有管理此环境成员的权限"
      type="warning"
      :closable="false"
      show-icon
    />

    <el-table
      v-else
      v-loading="loading"
      :data="members"
      empty-text="当前环境暂无成员"
    >
      <el-table-column
        label="用户"
        min-width="180"
      >
        <template #default="{ row }">
          <strong>{{ row.user.display_name || row.user.username }}</strong>
          <div class="member-username">
            {{ row.user.username }}
          </div>
        </template>
      </el-table-column>
      <el-table-column
        label="当前环境角色"
        min-width="260"
      >
        <template #default="{ row }">
          <el-tag
            v-for="role in row.roles"
            :key="String(role.id)"
            class="member-role"
            type="info"
          >
            {{ role.name }}
          </el-tag>
          <span v-if="!row.roles || row.roles.length === 0">未分配</span>
        </template>
      </el-table-column>
      <el-table-column
        label="操作"
        width="120"
        fixed="right"
      >
        <template #default="{ row }">
          <el-button
            type="text"
            @click="openEdit(row)"
          >
            分配角色
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      :title="editingMember ? '分配环境角色' : '添加环境成员'"
      :visible.sync="dialogVisible"
      width="560px"
      :close-on-click-modal="false"
    >
      <el-form label-position="top">
        <el-form-item label="用户 ID">
          <el-input
            v-model.trim="memberForm.userId"
            :disabled="Boolean(editingMember)"
            inputmode="numeric"
            placeholder="输入已有系统用户的 ID"
          />
          <p class="form-help">
            环境管理员不能读取或修改全局用户资料。请填写已有用户 ID。
          </p>
        </el-form-item>
        <el-form-item label="环境角色">
          <el-checkbox-group v-model="memberForm.roleIds">
            <el-checkbox
              v-for="role in availableRoles"
              :key="String(role.id)"
              :label="role.id"
            >
              {{ role.name }}（{{ role.key }}）
            </el-checkbox>
          </el-checkbox-group>
          <p
            v-if="availableRoles.length === 0"
            class="form-help"
          >
            后端没有返回可分配角色。
          </p>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">
          取消
        </el-button>
        <el-button
          type="primary"
          :loading="saving"
          :disabled="availableRoles.length === 0"
          @click="saveMember"
        >
          保存角色
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script>
import {
  listEnvironmentMembers,
  listRoles,
  updateEnvironmentMemberRoles,
} from '@/api/roles'

import { actorAccess, assignableRoles } from './access'
import { actionErrorMessage, requireList } from './errors'

export default {
  name: 'MembersPage',

  data() {
    return {
      members: [],
      roles: [],
      loading: false,
      saving: false,
      loadSequence: 0,
      errorMessage: '',
      dialogVisible: false,
      editingMember: null,
      memberForm: {
        userId: '',
        roleIds: [],
      },
    }
  },

  computed: {
    systemAdmin() {
      return this.$store.getters['session/systemAdmin']
    },
    currentEnvironment() {
      return this.$store.getters['environment/current']
    },
    currentEnvironmentId() {
      return this.currentEnvironment?.id ?? null
    },
    canManage() {
      return actorAccess({
        systemAdmin: this.systemAdmin,
        permissions: this.$store.state.environment.permissions,
      }).manageMembers
    },
    availableRoles() {
      return assignableRoles(this.roles, this.systemAdmin)
    },
  },

  watch: {
    currentEnvironmentId: {
      immediate: true,
      handler() {
        if (this.canManage && this.currentEnvironmentId) {
          this.load()
        } else {
          this.members = []
          this.roles = []
        }
      },
    },
  },

  methods: {
    async load() {
      const environmentId = this.currentEnvironmentId
      if (!environmentId) {
        return
      }
      this.loadSequence += 1
      const sequence = this.loadSequence
      this.loading = true
      this.errorMessage = ''
      try {
        const [memberReply, roleReply] = await Promise.all([
          listEnvironmentMembers(environmentId),
          listRoles(),
        ])
        if (sequence !== this.loadSequence) {
          return
        }
        this.members = requireList(memberReply, 'environment member list')
        this.roles = requireList(roleReply, 'role list')
      } catch (error) {
        if (sequence === this.loadSequence) {
          this.errorMessage = actionErrorMessage(error, '读取环境成员')
        }
      } finally {
        if (sequence === this.loadSequence) {
          this.loading = false
        }
      }
    },
    openAdd() {
      this.editingMember = null
      this.memberForm = { userId: '', roleIds: [] }
      this.errorMessage = ''
      this.dialogVisible = true
    },
    openEdit(member) {
      this.editingMember = member
      this.memberForm = {
        userId: String(member.user.id),
        roleIds: Array.isArray(member.roles)
          ? member.roles.map((role) => role.id)
          : [],
      }
      this.errorMessage = ''
      this.dialogVisible = true
    },
    async saveMember() {
      if (!/^[1-9]\d*$/.test(this.memberForm.userId)) {
        this.errorMessage = '用户 ID 必须是正整数'
        return
      }
      this.saving = true
      this.errorMessage = ''
      try {
        await updateEnvironmentMemberRoles(
          this.currentEnvironmentId,
          this.memberForm.userId,
          this.memberForm.roleIds,
        )
        this.dialogVisible = false
        await this.load()
        this.$message.success('环境成员角色已更新')
      } catch (error) {
        this.errorMessage = actionErrorMessage(error, '保存环境成员角色')
      } finally {
        this.saving = false
      }
    },
  },
}
</script>

<style scoped src="./management-card.css"></style>

<style scoped>
.member-username,
.form-help {
  margin: 4px 0 0;
  color: #6b7280;
  font-size: 13px;
}

.member-role {
  margin: 2px 6px 2px 0;
}
</style>
