<template>
  <div class="app-shell">
    <header class="app-shell__header">
      <router-link
        class="app-shell__brand"
        to="/"
      >
        Kirby
      </router-link>

      <div class="app-shell__actions">
        <EnvironmentTag :environment="currentEnvironment" />
        <el-select
          v-if="environments.length"
          :value="currentEnvironmentId"
          :loading="switching"
          aria-label="当前环境"
          size="small"
          @change="switchEnvironment"
        >
          <el-option
            v-for="environment in environments"
            :key="String(environment.id)"
            :label="environment.name"
            :value="environment.id"
            :disabled="!environment.enabled"
          />
        </el-select>
        <span class="app-shell__user">{{ displayName }}</span>
        <el-button
          type="text"
          @click="logout"
        >
          退出
        </el-button>
      </div>
    </header>

    <main class="app-shell__content">
      <router-view />
    </main>
  </div>
</template>

<script>
import EnvironmentTag from '@/components/EnvironmentTag'

export default {
  name: 'AppLayout',

  components: {
    EnvironmentTag,
  },

  computed: {
    environments() {
      return this.$store.state.environment.available
    },
    currentEnvironmentId() {
      return this.$store.state.environment.currentId
    },
    currentEnvironment() {
      return this.$store.getters['environment/current']
    },
    switching() {
      return this.$store.state.environment.switching
    },
    displayName() {
      const user = this.$store.state.session.user
      return user?.display_name || user?.username || ''
    },
  },

  methods: {
    async switchEnvironment(environmentId) {
      try {
        await this.$store.dispatch('environment/select', environmentId)
      } catch (error) {
        this.$message.error(error.message)
      }
    },
    async logout() {
      let failure = null
      try {
        await this.$store.dispatch('session/logout')
      } catch (error) {
        failure = error
      }
      await this.$router.replace({ name: 'login' })
      if (failure) {
        this.$message.error(`服务端退出失败：${failure.message}`)
      }
    },
  },
}
</script>

<style scoped>
.app-shell {
  min-height: 100vh;
  background: #f3f5f8;
}

.app-shell__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 64px;
  padding: 0 24px;
  border-bottom: 1px solid #e5e7eb;
  background: #fff;
}

.app-shell__brand {
  color: #111827;
  font-size: 22px;
  font-weight: 700;
  text-decoration: none;
}

.app-shell__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.app-shell__user {
  color: #4b5563;
  font-size: 14px;
}

.app-shell__content {
  padding: 32px;
}

</style>
