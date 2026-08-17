<template>
  <main class="status-page">
    <section>
      <p class="status-page__code">
        403
      </p>
      <h1>没有访问权限</h1>
      <p>当前账号在所选环境中没有执行此操作的权限。</p>
      <router-link to="/">
        返回首页
      </router-link>
    </section>
  </main>
</template>

<script>
export default {
  name: 'ForbiddenPage',
}
</script>

<style scoped>
.status-page {
  display: grid;
  min-height: 100vh;
  padding: 24px;
  text-align: center;
  place-items: center;
}

.status-page__code {
  margin: 0;
  color: #dc2626;
  font-size: 52px;
  font-weight: 800;
}

.status-page h1 {
  margin: 8px 0;
}

.status-page p:not(.status-page__code) {
  color: #6b7280;
}
</style>
