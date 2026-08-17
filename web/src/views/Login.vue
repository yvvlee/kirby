<template>
  <main
    class="login-page"
    aria-labelledby="login-title"
  >
    <section class="login-card">
      <p class="login-card__eyebrow">
        Kirby
      </p>
      <h1 id="login-title">
        登录配置管理平台
      </h1>

      <el-alert
        v-if="errorMessage"
        :title="errorMessage"
        type="error"
        show-icon
        :closable="false"
      />

      <el-form
        ref="loginForm"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.native.prevent="submit"
      >
        <el-form-item
          label="用户名"
          prop="username"
        >
          <el-input
            v-model.trim="form.username"
            autocomplete="username"
            autofocus
          />
        </el-form-item>
        <el-form-item
          label="密码"
          prop="password"
        >
          <el-input
            v-model="form.password"
            type="password"
            autocomplete="current-password"
            show-password
          />
        </el-form-item>
        <el-button
          class="login-card__submit"
          native-type="submit"
          type="primary"
          :loading="busy"
        >
          登录
        </el-button>
      </el-form>
    </section>
  </main>
</template>

<script>
function safeRedirect(value) {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//')
    ? value
    : '/'
}

export default {
  name: 'LoginPage',

  data() {
    return {
      form: {
        username: '',
        password: '',
      },
      errorMessage: '',
      rules: {
        username: [
          { required: true, message: '请输入用户名', trigger: 'blur' },
        ],
        password: [
          { required: true, message: '请输入密码', trigger: 'blur' },
        ],
      },
    }
  },

  computed: {
    busy() {
      return this.$store.state.session.busy
    },
  },

  methods: {
    async submit() {
      this.errorMessage = ''
      await this.$refs.loginForm.validate()
      try {
        await this.$store.dispatch('session/login', this.form)
        await this.$router.replace(safeRedirect(this.$route.query.redirect))
      } catch (error) {
        this.errorMessage =
          error.response?.data?.message || error.message || '登录失败'
      }
    },
  },
}
</script>

<style scoped>
.login-page {
  display: grid;
  min-height: 100vh;
  padding: 24px;
  background: #f3f5f8;
  place-items: center;
}

.login-card {
  width: min(100%, 420px);
  padding: 40px;
  border: 1px solid #e5e7eb;
  border-radius: 14px;
  background: #fff;
  box-shadow: 0 18px 45px rgb(15 23 42 / 8%);
}

.login-card__eyebrow {
  margin: 0 0 8px;
  color: #2563eb;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.login-card h1 {
  margin: 0 0 28px;
  color: #111827;
  font-size: 26px;
}

.login-card .el-alert {
  margin-bottom: 20px;
}

.login-card__submit {
  width: 100%;
}
</style>
