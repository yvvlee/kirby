import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'

import { defineConfig } from 'vite'
import type { Plugin } from 'vite'
import react from '@vitejs/plugin-react'

const apiTarget = process.env.KIRBY_DEV_API_TARGET || 'http://127.0.0.1:8000'

const monacoNlsPath = fileURLToPath(
  new URL('./node_modules/monaco-editor/esm/vs/nls/lang/zh-cn.js', import.meta.url),
)
const monacoNlsSource = readFileSync(monacoNlsPath, 'utf8')
const monacoNlsMatch = monacoNlsSource.match(
  /globalThis\._VSCODE_NLS_MESSAGES=(\[[\s\S]*?\]);\s*globalThis\._VSCODE_NLS_LANGUAGE=/,
)
if (!monacoNlsMatch) throw new Error('Could not read Monaco Chinese NLS messages')
const monacoZhMessages = JSON.parse(monacoNlsMatch[1]) as string[]

function isolateMonacoLocale(): Plugin {
  return {
    name: 'kirby-monaco-locale-graph',
    enforce: 'pre',
    resolveId(source, importer) {
      if (!importer?.includes('/node_modules/monaco-editor/esm/vs/')) return null
      const locale = importer.match(/[?&]kirbyLocale=(zh-CN|en-US)/)?.[1]
      if (!locale) return null
      if (!source.startsWith('.') && !source.startsWith('monaco-editor/')) return null
      if (/[?&]kirbyLocale=/.test(source)) return null
      const separator = source.includes('?') ? '&' : '?'
      return this.resolve(
        `${source}${separator}kirbyLocale=${locale}`,
        importer,
        { skipSelf: true },
      )
    },
    transform(code, id) {
      if (!id.includes('/node_modules/monaco-editor/esm/vs/nls.js?')) return null
      const locale = id.match(/[?&]kirbyLocale=(zh-CN|en-US)/)?.[1]
      if (!locale) return null
      const messages = locale === 'zh-CN' ? JSON.stringify(monacoZhMessages) : 'undefined'
      const language = JSON.stringify(locale === 'zh-CN' ? 'zh-cn' : 'en')
      return code
        .replace('return globalThis._VSCODE_NLS_MESSAGES;', `return ${messages};`)
        .replace('return globalThis._VSCODE_NLS_LANGUAGE;', `return ${language};`)
    },
  }
}

export default defineConfig({
  plugins: [isolateMonacoLocale(), react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 15173,
    proxy: {
      '/api': {
        target: apiTarget,
      },
    },
  },
  preview: {
    host: '0.0.0.0',
    port: 15174,
  },
})
