import { afterEach, expect, it } from 'vitest'
import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosHeaders } from 'axios'
import { createApiClient } from '@/api/client'
import { getLocale, setLocale, t, useTranslation } from './index'
import LanguageSwitcher from './LanguageSwitcher'

afterEach(() => { act(() => setLocale('zh-CN')) })

function Example() {
  const { t } = useTranslation()
  return <><LanguageSwitcher /><span>{t('全局管理')}</span><input aria-label="draft" defaultValue="" /></>
}

it('updates visible text without remounting an unsaved form and persists the language', async () => {
  render(<Example />)
  await userEvent.type(screen.getByRole('textbox', { name: 'draft' }), 'unsaved value')
  act(() => setLocale('en-US'))
  expect(screen.getByText('Administration')).toBeInTheDocument()
  expect(screen.getByRole('textbox', { name: 'draft' })).toHaveValue('unsaved value')
  expect(localStorage.getItem('kirby.locale')).toBe('en-US')
  expect(document.documentElement.lang).toBe('en-US')
  expect(document.title).toBe('Kirby Configuration Platform')
  act(() => setLocale('zh-CN'))
  expect(screen.getByText('全局管理')).toBeInTheDocument()
})

it('substitutes values without translating project names or special characters', () => {
  setLocale('en-US')
  expect(t('服务端退出失败：{{error}}', { error: '<项目>{{unknown}}' })).toBe('Server sign-out failed: <项目>{{unknown}}')
})

it('sends the current language on subsequent API calls', async () => {
  const requested: unknown[] = []
  const client = createApiClient({
    adapter: async (config) => {
      requested.push(config.headers.get('Accept-Language'))
      return {
        data: {},
        status: 200,
        statusText: 'OK',
        headers: new AxiosHeaders(),
        config,
      }
    },
  })
  setLocale('en-US')
  await client.get('/setup/status')
  setLocale('zh-CN')
  await client.get('/setup/status')
  expect(requested).toEqual(['en-US', 'zh-CN'])
  expect(getLocale()).toBe('zh-CN')
})

it('keeps raised application errors translatable without changing parameter values', async () => {
  const { localizedError } = await import('./errors')
  const error = localizedError('找不到模型: {{key}}', {
    key: '用户模型',
  }, TypeError)
  expect(error).toBeInstanceOf(TypeError)
  expect(error.message).toBe('找不到模型: 用户模型')
  setLocale('en-US')
  expect(error.message).toBe('Model not found: 用户模型')
  setLocale('zh-CN')
  expect(error.message).toBe('找不到模型: 用户模型')
})

it('localizes password rules without changing what passwords are accepted', async () => {
  const { passwordPolicyError, newPasswordRules } = await import('@/auth/password-policy')
  expect(passwordPolicyError('short')).toBe('密码长度须为 8 至 32 个字符')
  setLocale('en-US')
  expect(passwordPolicyError('short')).toBe('Password must be 8 to 32 characters long')
  expect(newPasswordRules()[0]).toMatchObject({
    message: 'Enter a password',
  })
  expect(passwordPolicyError('Abcd1234')).toBeNull()
  setLocale('zh-CN')
  expect(passwordPolicyError('Abcd1234')).toBeNull()
})
