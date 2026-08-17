<template>
  <section
    class="home-view"
    aria-labelledby="welcome-title"
  >
    <p>Kirby</p>
    <h1 id="welcome-title">
      配置管理平台
    </h1>
    <p v-if="currentEnvironment">
      当前环境：{{ currentEnvironment.name }}
    </p>
    <p v-else>
      当前账号没有可用环境。
    </p>
  </section>
</template>

<script>
export default {
  name: 'HomeView',

  computed: {
    currentEnvironment() {
      return this.$store.getters['environment/current']
    },
  },
}
</script>

<style scoped>
.home-view {
  max-width: 720px;
  padding: 40px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #fff;
}

.home-view p:first-child {
  margin: 0 0 8px;
  color: #2563eb;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.home-view h1 {
  margin: 0 0 16px;
}

.home-view p:last-child {
  margin: 0;
  color: #6b7280;
}
</style>
