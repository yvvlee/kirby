import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const apiTarget = process.env.KIRBY_DEV_API_TARGET || 'http://127.0.0.1:8000'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 15173,
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
    port: 15174,
  },
})
