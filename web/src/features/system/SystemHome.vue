<template>
  <section class="system-card">
    <h2>可用管理功能</h2>
    <p v-if="availableCount">
      当前账号可以使用 {{ availableCount }} 项管理功能。请选择上方入口。
    </p>
    <el-alert
      v-else
      title="当前账号没有环境或系统管理权限"
      description="如需管理成员，请让环境管理员分配环境成员管理权限。"
      type="info"
      :closable="false"
      show-icon
    />
  </section>
</template>

<script>
import { actorAccess } from './access'

export default {
  name: 'SystemHome',

  computed: {
    availableCount() {
      const access = actorAccess({
        systemAdmin: this.$store.getters['session/systemAdmin'],
        permissions: this.$store.state.environment.permissions,
      })
      return Object.values(access).filter(Boolean).length
    },
  },
}
</script>

<style scoped>
.system-card {
  padding: 28px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
}

.system-card h2 {
  margin-top: 0;
}

.system-card p {
  margin-bottom: 0;
  color: #4b5563;
}
</style>
