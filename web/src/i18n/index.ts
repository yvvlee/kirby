import { useCallback, useSyncExternalStore } from 'react'
import { configCenterMessages } from './config-center'
import { uiMessages } from './ui'

export type Locale = 'zh-CN' | 'en-US'
export type TranslationParams = Record<string, string | number>
const storageKey = 'kirby.locale'
const listeners = new Set<() => void>()
const coreMessages: Record<string, string> = {
  '语言': 'Language',
  '环境不可用：{{id}}': 'Environment unavailable: {{id}}',
  '环境已禁用：{{id}}': 'Environment disabled: {{id}}',
  '网络连接失败，请稍后重试': 'Network connection failed. Please try again.',
  '请求超时，请稍后重试': 'The request timed out. Please try again.',
  '全局管理': 'Administration',
  '退出': 'Sign out',
  '服务端退出失败：{{error}}': 'Server sign-out failed: {{error}}',
  '请求失败': 'Request failed',
  '无法读取初始化状态': 'Unable to load setup status',
  '请检查连接后重试': 'Check your connection and try again',
  '重试': 'Retry',
  '无法恢复会话': 'Unable to restore your session',
  '重新加载': 'Reload',
  '环境加载失败': 'Unable to load environments',
  '初始化状态响应无效': 'Invalid setup status response',
  'Kirby 配置管理平台': 'Kirby Configuration Platform',
}
const messages = {
  ...coreMessages,
  ...configCenterMessages,
  ...uiMessages,
}
let locale: Locale = readLocale()

function readLocale(): Locale {
  const saved = localStorage.getItem(storageKey)
  return saved === 'en-US' ? 'en-US' : 'zh-CN'
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

/**
 * Reads the language shared by rendering and API requests.
 * Parameters: none.
 * Returns: the selected supported locale.
 * Throws: none.
 */
export function getLocale(): Locale {
  return locale
}

/**
 * Persists an explicit choice without remounting forms or losing edits.
 * Parameters: next is a supported locale.
 * Returns: nothing.
 * Throws: storage errors when the preference cannot be persisted.
 */
export function setLocale(next: Locale): void {
  localStorage.setItem(storageKey, next)
  locale = next
  document.documentElement.lang = next
  document.title = t('Kirby 配置管理平台')
  listeners.forEach((listener) => listener())
}

/**
 * Translates application-owned text while leaving interpolation values untouched.
 * Parameters: text is a Chinese source key; params contains named values.
 * Returns: localized text with named placeholders substituted.
 * Throws: none.
 */
export function t(text: string, params: TranslationParams = {}): string {
  return translate(text, params, locale)
}

function translate(text: string, params: TranslationParams, language: Locale): string {
  const translated = language === 'en-US' ? (messages[text] ?? text) : text
  return translated.replace(/\{\{(\w+)\}\}/g, (placeholder, name: string) =>
    Object.hasOwn(params, name) ? String(params[name]) : placeholder,
  )
}

/**
 * Subscribes a component to language changes, including pure helpers it calls.
 * Parameters: none.
 * Returns: the translation function and active locale.
 * Throws: none.
 */
export function useTranslation() {
  const current = useSyncExternalStore(subscribe, getLocale, getLocale)
  const translateCurrent = useCallback(
    (text: string, params: TranslationParams = {}) => translate(text, params, current),
    [current],
  )
  return {
    t: translateCurrent,
    locale: current,
  }
}
