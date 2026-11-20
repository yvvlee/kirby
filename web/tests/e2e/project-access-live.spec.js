import { mkdir } from 'node:fs/promises'
import { test, expect } from '@playwright/test'

// Run after API verification has enabled registration on the disposable server.
test.beforeAll(async ({ request }) => {
  if (!process.env.KIRBY_LOCAL_WEB_URL) return
  const login = await request.post('/api/auth/login', { data: { username: 'admin', password: 'kirby-e2e-admin-password' }, headers: { Origin: process.env.KIRBY_LOCAL_WEB_URL } })
  expect(login.ok()).toBeTruthy()
  const session = await login.json()
  const updated = await request.put('/api/admin/system-settings', { data: { allow_self_registration: true }, headers: { Authorization: `Bearer ${session.access_token}`, Origin: process.env.KIRBY_LOCAL_WEB_URL } })
  expect(updated.ok()).toBeTruthy()
})

test('ordinary user creates a project and its first environment', async ({ page }) => {
  const name = `browser${Date.now()}`
  await page.goto('/register')
  await page.getByLabel('用户名', { exact: true }).fill(name)
  await page.getByLabel('昵称', { exact: true }).fill(name)
  await page.getByLabel('邮箱', { exact: true }).fill(`${name}@example.test`)
  await page.getByLabel('密码', { exact: true }).fill('BrowserTestPassword123!')
  await page.getByLabel('确认密码', { exact: true }).fill('BrowserTestPassword123!')
  await page.getByRole('button', { name: /注\s*册/ }).click()
  await expect(page).toHaveURL(/\/projects$/)
  await page.getByRole('button', { name: '创建项目', exact: true }).click()
  await page.getByLabel('项目标识', { exact: true }).fill(name)
  await page.getByLabel('项目名称', { exact: true }).fill(name)
  await page.getByRole('button', { name: /保\s*存/ }).click()
  await page.getByRole('heading', { name, exact: true }).click()
  await expect(page).toHaveURL(/\/projects\/\d+\/environments$/)
  await page.getByRole('button', { name: '新建环境', exact: true }).click()
  await page.getByLabel('环境标识', { exact: true }).fill('dev')
  await page.getByLabel('名称', { exact: true }).fill('Development')
  await page.getByRole('button', { name: /保\s*存/ }).click()
  await expect(page.getByRole('cell', { name: 'Development', exact: true })).toBeVisible()
  await expect(page.getByRole('dialog')).toBeHidden()
  await mkdir('../.local/access-audit', { recursive: true })
  await page.screenshot({ path: '../.local/access-audit/project-environments.png', fullPage: true })
})

for (const mode of ['login', 'register']) {
  test(`invitation returns after ${mode} and requires confirmation`, async ({ page, request, baseURL }) => {
    const suffix = Date.now().toString()
    const password = 'InvitationPassword123!'
    const createAccount = async (name) => {
      const response = await request.post('/api/auth/register', {
        headers: { Origin: baseURL },
        data: { username: name, nickname: name, email: `${name}@example.test`, password },
      })
      expect(response.ok(), await response.text()).toBeTruthy()
      return response.json()
    }
    const owner = await createAccount(`owner${suffix}`)
    const headers = { Origin: baseURL, Authorization: `Bearer ${owner.access_token}` }
    const created = await request.post('/api/admin/projects', { headers, data: { key: `Invite${suffix}`, name: `Invite ${suffix}` } })
    expect(created.ok()).toBeTruthy()
    const { project } = await created.json()
    const environment = await request.post(`/api/admin/projects/${project.id}/environments`, { headers, data: { key: 'dev', name: 'Development' } })
    expect(environment.ok()).toBeTruthy()
    const invitation = await request.post(`/api/admin/projects/${project.id}/invitations`, { headers, data: { role: 'GUEST' } })
    expect(invitation.ok()).toBeTruthy()
    const { token } = await invitation.json()
    const username = `guest${suffix}`
    if (mode === 'login') await createAccount(username)
    await page.goto(`/invitations/${token}`)
    await page.getByRole('link', { name: mode === 'login' ? '登录后确认' : '注册后确认' }).click()
    await page.getByLabel(mode === 'login' ? '用户名或邮箱' : '用户名', { exact: true }).fill(username)
    await page.getByLabel('密码', { exact: true }).fill(password)
    if (mode === 'register') {
      await page.getByLabel('昵称', { exact: true }).fill(username)
      await page.getByLabel('邮箱', { exact: true }).fill(`${username}@example.test`)
      await page.getByLabel('确认密码', { exact: true }).fill(password)
    }
    await page.getByRole('button', { name: mode === 'login' ? /登\s*录/ : /注\s*册/ }).click()
    await expect(page).toHaveURL(`/invitations/${token}`)
    await expect(page.getByRole('button', { name: '确认加入项目' })).toBeVisible()
    const before = await request.get(`/api/admin/projects/${project.id}/members`, { headers })
    expect((await before.json()).list).toHaveLength(1)
    await page.getByRole('button', { name: '确认加入项目' }).click()
    await expect(page).toHaveURL('/projects')
    await expect(page.getByRole('heading', { name: `Invite ${suffix}`, exact: true })).toBeVisible()
    const after = await request.get(`/api/admin/projects/${project.id}/members`, { headers })
    expect((await after.json()).list).toHaveLength(2)
    await page.getByRole('heading', { name: `Invite ${suffix}`, exact: true }).click()
    await expect(page).toHaveURL(new RegExp(`/projects/${project.id}/configs$`))
    await expect(page.getByRole('columnheader', { name: '配置标识' })).toBeVisible()
  })
}

test('API Key management remains reachable without configuration read', async ({ page, request, baseURL }) => {
  const name = `keyonly${Date.now()}`
  const password = 'KeyOnlyPassword123!'
  const account = async (username) => {
    const response = await request.post('/api/auth/register', {
      headers: { Origin: baseURL },
      data: { username, nickname: username, email: `${username}@example.test`, password },
    })
    expect(response.ok()).toBeTruthy()
    return response.json()
  }
  const owner = await account(`owner${name}`)
  const guest = await account(name)
  const headers = { Origin: baseURL, Authorization: `Bearer ${owner.access_token}` }
  const projectResponse = await request.post('/api/admin/projects', { headers, data: { key: name, name } })
  const { project } = await projectResponse.json()
  const environmentResponse = await request.post(`/api/admin/projects/${project.id}/environments`, { headers, data: { key: 'dev', name: 'Development' } })
  const { environment } = await environmentResponse.json()
  const added = await request.post(`/api/admin/projects/${project.id}/members`, { headers, data: { user_id: guest.user.id, role: 'GUEST' } })
  expect(added.ok()).toBeTruthy()
  const updated = await request.put(`/api/admin/environments/${environment.id}/permissions`, { headers, data: { roles: [{ role: 'GUEST', permissions: ['API_KEY_MANAGE'] }] } })
  expect(updated.ok(), await updated.text()).toBeTruthy()
  await page.goto('/login')
  await page.getByLabel('用户名或邮箱').fill(name)
  await page.getByLabel('密码', { exact: true }).fill(password)
  await page.getByRole('button', { name: /登\s*录/ }).click()
  await page.getByRole('heading', { name, exact: true }).click()
  await expect(page.getByText('当前环境未授予配置读取权限')).toBeVisible()
  await page.getByRole('button', { name: 'API Key', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
})
