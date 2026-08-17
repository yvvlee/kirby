import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue2'

const apiTarget = process.env.KIRBY_DEV_API_TARGET || 'http://127.0.0.1:8000'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api/assets/upload': { target: apiTarget },
      '/api/assets/objects': { target: apiTarget },
      '/api': {
        target: apiTarget,
        cookiePathRewrite: { '/auth': '/api/auth' },
        rewrite: (path) => path.replace(/^\/api(?=\/|$)/, '') || '/',
      },
    },
  },
  preview: {
    host: '0.0.0.0',
    port: 4173,
  },
})
