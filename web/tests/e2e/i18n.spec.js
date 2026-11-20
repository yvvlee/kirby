import { expect, test } from '@playwright/test'

async function switchLanguage(page, label) {
  await page.getByTestId('language-selector').click()
  await page.locator('.ant-select-dropdown:visible').getByText(label, { exact: true }).click()
}

async function createEditorFixture(request, token, key) {
  const projectReply = await request.post('/api/admin/projects', {
    data: { key, name: key, description: 'Monaco locale E2E fixture' },
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(projectReply.ok(), await projectReply.text()).toBeTruthy()
  const project = (await projectReply.json()).project

  const environmentReply = await request.post(`/api/admin/projects/${project.id}/environments`, {
    data: { key: 'locale-e2e', name: 'Locale E2E' },
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(environmentReply.ok(), await environmentReply.text()).toBeTruthy()
  const environmentId = (await environmentReply.json()).environment.id

  const configReply = await request.post(`/api/admin/environments/${environmentId}/config/create`, {
    data: {
      environment_id: environmentId,
      project_id: project.id,
      key: 'MonacoLocaleText',
      description: 'Monaco locale test value',
    },
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(configReply.ok(), await configReply.text()).toBeTruthy()

  return { project }
}

async function expectFindWidgetLocale(page, language) {
  const findInput = language === 'zh-CN'
    ? page.getByRole('textbox', { name: '查找', exact: true })
    : page.getByRole('textbox', { name: 'Find', exact: true })
  await expect(findInput).toBeVisible()

  const dialogName = language === 'zh-CN' ? '查找/替换' : 'Find / Replace'
  await expect(page.getByRole('dialog', { name: dialogName })).toBeVisible()
}

async function openFindWidget(page) {
  const widget = page.locator('.find-widget')
  await page.keyboard.press('Control+f')
  if (await widget.count() === 0) await page.keyboard.press('Meta+f')
}

// Uses only the disposable server started by scripts/test-local-dev.sh.
test('language choice persists across reloads and covers public and administrator pages', async ({ page }) => {
  await page.goto('/login')
  await switchLanguage(page, 'English')
  await page.getByLabel('Username or email', { exact: true }).fill('admin')
  await page.getByLabel('Password', { exact: true }).fill('kirby-e2e-admin-password')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/projects$/)
  await expect(page.getByRole('button', { name: 'Create project', exact: true })).toBeVisible()
  await page.reload()
  await page.getByRole('button', { name: 'Create project', exact: true }).click()
  await page.getByLabel('Project name', { exact: true }).fill('未保存的项目名')
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeHidden()
  await switchLanguage(page, '简体中文')
  await expect(page.getByRole('button', { name: '创建项目', exact: true })).toBeVisible()
  await page.getByRole('button', { name: '全局管理', exact: true }).click()
  await page.getByRole('link', { name: '系统设置', exact: true }).click()
  await expect(page.getByText('允许自行注册', { exact: true })).toBeVisible()
  await switchLanguage(page, 'English')
  await expect(page.getByRole('heading', { name: 'System settings', exact: true })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('lang', 'en-US')
})

test('Monaco editor menu and find widget follow locale without losing the preview text', async ({ page, request }) => {
  await page.goto('/login')
  const loginResponse = page.waitForResponse((response) => response.url().endsWith('/api/auth/login') && response.request().method() === 'POST')
  await page.getByLabel('用户名或邮箱', { exact: true }).fill('admin')
  await page.getByLabel('密码', { exact: true }).fill('kirby-e2e-admin-password')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  const token = (await (await loginResponse).json()).access_token
  expect(token).toBeTruthy()

  const key = `MonacoLocale${Date.now()}`
  await createEditorFixture(request, token, key)
  await page.reload()
  await page.goto('/projects')
  await page.getByRole('link', { name: new RegExp(key) }).click()
  await expect(page.getByRole('button', { name: '创建配置' })).toBeVisible()
  await page.getByRole('button', { name: '查看详情', exact: true }).click()
  await expect(page.locator('.monaco-editor').first()).toBeVisible()

  const marker = `unsaved locale marker ${Date.now()}`
  await page.locator('.form-pane input').first().fill(marker)
  await page.getByRole('button', { name: '刷新 JSON 预览', exact: true }).click()
  const editor = page.locator('.json-pane .monaco-editor').first()
  await expect(editor.locator('.view-lines')).toContainText(marker)

  await editor.locator('.view-line').first().click()
  await editor.evaluate((node) => node.querySelector('.native-edit-context')?.focus())
  await openFindWidget(page)
  await expectFindWidgetLocale(page, 'zh-CN')
  await expect(editor.locator('.view-lines')).toContainText(marker)
  await page.keyboard.press('Escape')

  await editor.locator('.view-line').first().click({ button: 'right' })
  const contextMenu = page.locator('.monaco-menu-container:visible')
  await expect(contextMenu.getByText('复制', { exact: true })).toBeVisible()
  await page.keyboard.press('Escape')

  await switchLanguage(page, 'English')
  await expect(page.locator('.monaco-editor .native-edit-context[aria-label="Editor content"]')).toHaveCount(1)
  await expect(page.locator('.monaco-editor .native-edit-context[aria-label="编辑器内容"]')).toHaveCount(0)
  const englishEditor = page.locator('.json-pane .monaco-editor').first()
  await expect(englishEditor.locator('.view-lines')).toContainText(marker)
  await englishEditor.locator('.view-line').first().click()
  await englishEditor.evaluate((node) => node.querySelector('.native-edit-context')?.focus())
  await openFindWidget(page)
  await expectFindWidgetLocale(page, 'en-US')
  await expect(englishEditor.locator('.view-lines')).toContainText(marker)
  await page.keyboard.press('Escape')
  await englishEditor.locator('.view-line').first().click({ button: 'right' })
  const englishContextMenu = page.locator('.monaco-menu-container:visible')
  await expect(englishContextMenu.getByText('Copy', { exact: true })).toBeVisible()
  await expect(englishContextMenu.getByText('复制', { exact: true })).toHaveCount(0)
  await expect(englishEditor.locator('.view-lines')).toContainText(marker)
})

test('API error language changes without changing the error code', async ({ request }) => {
  const chinese = await request.get('/api/admin/projects', { headers: { 'Accept-Language': 'zh-CN' } })
  const english = await request.get('/api/admin/projects', { headers: { 'Accept-Language': 'en-US' } })
  expect(chinese.status()).toBe(401)
  expect(english.status()).toBe(401)
  const zh = await chinese.json()
  const en = await english.json()
  expect(zh.reason).toBe(en.reason)
  expect(zh.message).not.toBe(en.message)
  expect(zh.message).toMatch(/[\p{Script=Han}]/u)
  expect(en.message).not.toMatch(/[\p{Script=Han}]/u)
})
