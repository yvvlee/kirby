import { expect, test } from '@playwright/test'

const administrator = {
  username: 'setup-owner',
  display_name: '初始管理员',
  password: 'kirby-setup-admin-password',
}

async function fillSetup(page, username = administrator.username) {
  await page.getByRole('textbox', { name: '用户名' }).fill(username)
  await page.getByRole('textbox', { name: '显示名' }).fill(administrator.display_name)
  await page.getByLabel('密码', { exact: true }).fill(administrator.password)
  await page.getByLabel('确认密码', { exact: true }).fill(administrator.password)
  await page.getByRole('button', { name: '创建管理员' }).click()
}

test('creates the first administrator in an empty installation and logs in', async ({ page, request, browser }) => {
  const initialStatus = await request.get('/api/setup/status')
  expect(initialStatus.ok()).toBeTruthy()
  expect(await initialStatus.json()).toEqual({ initialized: false })

  await page.goto('/login')
  await expect(page).toHaveURL(/\/setup$/)
  await expect(page.getByRole('heading', { name: '创建初始管理员' })).toBeVisible()
  await page.goto('/system/users')
  await expect(page).toHaveURL(/\/setup$/)
  await expect(page.getByRole('heading', { name: '创建初始管理员' })).toBeVisible()

  const competingPage = await page.context().newPage()
  await competingPage.goto('/setup')
  await expect(competingPage.getByRole('heading', { name: '创建初始管理员' })).toBeVisible()

  const createResponse = page.waitForResponse((response) =>
    response.url().endsWith('/api/setup/admin') && response.request().method() === 'POST')
  await fillSetup(page)
  expect((await createResponse).ok()).toBeTruthy()
  await expect(page).toHaveURL(/\/login$/)
  await expect(page.getByText('管理员已创建，请登录')).toBeVisible()

  const conflictResponse = competingPage.waitForResponse((response) =>
    response.url().endsWith('/api/setup/admin') && response.request().method() === 'POST')
  await fillSetup(competingPage, 'second-owner')
  expect((await conflictResponse).status()).toBe(409)
  await expect(competingPage).toHaveURL(/\/login$/)
  await competingPage.close()

  const readyStatus = await request.get('/api/setup/status')
  expect(await readyStatus.json()).toEqual({ initialized: true })
  await page.getByRole('textbox', { name: '用户名' }).fill(administrator.username)
  await page.getByLabel('密码', { exact: true }).fill(administrator.password)
  const loginResponse = page.waitForResponse((response) =>
    response.url().endsWith('/api/auth/login') && response.request().method() === 'POST')
  await page.getByRole('button', { name: /登\s*录/ }).click()
  expect((await loginResponse).status()).toBe(200)
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', {
    name: '配置管理平台',
    exact: true,
  })).toBeVisible()

  await page.goto('/setup')
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByRole('heading', {
    name: '配置管理平台',
    exact: true,
  })).toBeVisible()

  const anonymousPage = await browser.newPage()
  const origin = new URL(page.url()).origin
  await anonymousPage.goto(`${origin}/setup`)
  await expect(anonymousPage).toHaveURL(/\/login$/)
  await expect(anonymousPage.getByRole('heading', { name: '登录配置管理平台' })).toBeVisible()
  await anonymousPage.close()
})
