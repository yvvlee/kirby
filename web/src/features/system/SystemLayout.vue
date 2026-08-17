<template>
  <section class="system-layout">
    <header class="system-layout__header">
      <div>
        <p class="system-layout__eyebrow">
          管理
        </p>
        <h1>环境与权限</h1>
      </div>
      <router-link to="/">
        返回配置平台
      </router-link>
    </header>

    <nav
      class="system-layout__nav"
      aria-label="环境与权限管理"
    >
      <router-link
        v-for="link in visibleLinks"
        :key="link.name"
        :to="{ name: link.name }"
      >
        {{ link.label }}
      </router-link>
    </nav>

    <router-view />
  </section>
</template>

<script>
import { actorAccess } from './access'

const navigation = [
  { name: 'system-home', label: '概览', capability: null },
  {
    name: 'system-environments',
    label: '环境',
    capability: 'manageEnvironments',
  },
  { name: 'system-users', label: '用户', capability: 'manageUsers' },
  { name: 'system-roles', label: '角色与权限', capability: 'manageRoles' },
  {
    name: 'environment-members',
    label: '当前环境成员',
    capability: 'manageMembers',
  },
]

export default {
  name: 'SystemLayout',

  computed: {
    access() {
      return actorAccess({
        systemAdmin: this.$store.getters['session/systemAdmin'],
        permissions: this.$store.state.environment.permissions,
      })
    },
    visibleLinks() {
      return navigation.filter(
        (link) => !link.capability || this.access[link.capability],
      )
    },
  },
}
</script>

<style scoped>
.system-layout {
  max-width: 1180px;
  margin: 0 auto;
}

.system-layout__header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 20px;
}

.system-layout__header h1,
.system-layout__eyebrow {
  margin: 0;
}

.system-layout__eyebrow {
  margin-bottom: 6px;
  color: #2563eb;
  font-size: 13px;
  font-weight: 700;
}

.system-layout__nav {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
  padding: 8px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  background: #fff;
}

.system-layout__nav a {
  padding: 8px 12px;
  border-radius: 7px;
  color: #4b5563;
  text-decoration: none;
}

.system-layout__nav .router-link-exact-active {
  background: #eff6ff;
  color: #1d4ed8;
  font-weight: 600;
}
</style>
