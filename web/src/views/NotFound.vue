<template>
  <main class="status-page">
    <section>
      <p class="status-page__code">
        404
      </p>
      <h1>页面不存在</h1>
      <p>请求的页面不存在或已经被移除。</p>
      <router-link to="/">
        返回首页
      </router-link>
    </section>
  </main>
</template>

<script>
export default {
  name: 'NotFoundPage',
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
  color: #2563eb;
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
