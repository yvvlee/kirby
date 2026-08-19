import { createHmac } from 'node:crypto'

import { expect, test } from '@playwright/test'

const adminCredential = 'kirby-e2e-admin-password'
const roleCredential = ['kirby', 'role', 'password'].join('-')
const testOrigin = new URL(process.env.KIRBY_LOCAL_WEB_URL || 'http://127.0.0.1:14173').origin
const signingMaterial = [
  'kirby-e2e-jwt-key-',
  '012345678901234567890123',
].join('')

function expiredAccessToken(userID) {
  const now = Math.floor(Date.now() / 1000)
  const encode = (value) => Buffer.from(JSON.stringify(value)).toString('base64url')
  const unsigned = [
    encode({ alg: 'HS256', typ: 'JWT', kid: 'e2e-primary' }),
    encode({
      iss: 'kirby-e2e', sub: String(userID), aud: ['kirby-admin'],
      exp: now - 60, nbf: now - 120, iat: now - 120,
      jti: 'e2e-expired-access-token', sid: 'e2e-expired-session',
    }),
  ].join('.')
  const signature = createHmac('sha256', signingMaterial)
    .update(unsigned)
    .digest('base64url')
  return `${unsigned}.${signature}`
}

async function api(request, token, method, path, data) {
  const response = await request.fetch(`/api${path}`, {
    method,
    data,
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  const text = await response.text()
  let body = null
  if (text) {
    try { body = JSON.parse(text) } catch { body = text }
  }
  expect(response.ok(), `${method} ${path}: ${text}`).toBeTruthy()
  return body
}

async function loginByAPI(request, username, password) {
  const response = await request.post('/api/auth/login', {
    data: { username, password },
    headers: { Origin: testOrigin },
  })
  expect(response.ok(), await response.text()).toBeTruthy()
  return response.json()
}

test('runs the standalone management and runtime journey against the real backend', async ({ page, request }) => {
  let loginReply
  const loginResponse = page.waitForResponse((response) => response.url().endsWith('/api/auth/login') && response.request().method() === 'POST')
  await page.goto('/login')
  await expect(page.getByRole('heading', { name: '登录配置管理平台' })).toBeVisible()
  const loginForm = page.getByRole('main')
  await loginForm.getByLabel('用户名').fill('admin')
  await loginForm.getByLabel('密码').fill(adminCredential)
  await page.getByRole('button', { name: /登\s*录/ }).click()
  loginReply = await (await loginResponse).json()
  const token = loginReply.access_token
  expect(token).toBeTruthy()
  await expect(page.getByRole('heading', { name: '配置管理平台' })).toBeVisible()

  await page.goto('/system/environments')
  await page.getByRole('button', { name: '新建环境' }).click()
  let dialog = page.getByRole('dialog', { name: '新建环境' })
  await dialog.getByPlaceholder('例如 production').fill('source')
  await dialog.getByLabel('名称').fill('Source')
  await dialog.getByLabel('说明').fill('E2E source environment')
  const sourceSaved = page.waitForResponse((response) => response.url().endsWith('/api/admin/environments') && response.request().method() === 'POST')
  await dialog.getByRole('button', { name: /保\s*存/ }).click()
  await sourceSaved
  await expect(dialog).toBeHidden()

  await page.getByRole('button', { name: '新建环境' }).click()
  dialog = page.getByRole('dialog', { name: '新建环境' })
  await dialog.getByPlaceholder('例如 production').fill('target')
  await dialog.getByLabel('名称').fill('Target')
  await dialog.getByLabel('说明').fill('E2E target environment')
  const targetSaved = page.waitForResponse((response) => response.url().endsWith('/api/admin/environments') && response.request().method() === 'POST')
  await dialog.getByRole('button', { name: /保\s*存/ }).click()
  await targetSaved
  await expect(dialog).toBeHidden()

  const environmentReply = await api(request, token, 'GET', '/admin/environments')
  const source = environmentReply.list.find((item) => item.key === 'source')
  const target = environmentReply.list.find((item) => item.key === 'target')
  expect(source?.id).toBeTruthy()
  expect(target?.id).toBeTruthy()

  await page.goto('/projects')
  await page.getByRole('button', { name: '创建项目' }).click()
  dialog = page.getByRole('dialog', { name: '创建项目' })
  await dialog.getByPlaceholder('例如 DemoConfig').fill('SourceProject')
  await dialog.getByLabel('项目名称').fill('Source Project')
  await dialog.getByLabel('项目描述').fill('E2E source')
  const sourceProjectSaved = page.waitForResponse((response) => response.url().endsWith(`/api/admin/environments/${source.id}/project/create`) && response.request().method() === 'POST')
  await dialog.getByRole('button', { name: /保\s*存/ }).click()
  const sourceProjectReply = await (await sourceProjectSaved).json()
  const targetProjectReply = await api(request, token, 'POST', `/admin/environments/${target.id}/project/create`, {
    environment_id: target.id, key: 'TargetProject', name: 'Target Project', description: 'E2E target',
  })
  const sourceProject = sourceProjectReply.project
  const targetProject = targetProjectReply.project

  await page.getByRole('link', { name: /Source Project/ }).click()
  await expect(page.getByRole('heading', { name: 'Source Project' })).toBeVisible()
  await expect(page.getByRole('button', { name: '创建配置' })).toBeVisible()

  let configReply = await api(request, token, 'POST', `/admin/environments/${source.id}/config/create`, {
    environment_id: source.id, project_id: sourceProject.id, key: 'Greeting', description: 'Runtime greeting',
  })
  configReply = await api(request, token, 'POST', `/admin/environments/${source.id}/config/updateValue`, {
    environment_id: source.id, id: configReply.config.id, value: '"hello from Kirby"', version: configReply.config.version,
  })

  const enumReply = await api(request, token, 'POST', `/admin/environments/${source.id}/enum/create`, {
    environment_id: source.id, config_id: configReply.config.id, key: 'Mode', name: 'Mode',
    description: 'E2E enum', values: [{ label: 'Enabled', value: 'ENABLED', description: '' }],
  })
  expect(enumReply.enum.key).toBe('Mode')

  const modelReply = await api(request, token, 'POST', `/admin/environments/${source.id}/structure/create`, {
    environment_id: source.id, config_id: configReply.config.id, key: 'Card', name: 'Card', description: 'E2E model',
  })
  expect(modelReply.structure.key).toBe('Card')

  const assetBody = Buffer.from('asset')
  const ticket = await api(request, token, 'POST', `/admin/environments/${source.id}/projects/${sourceProject.id}/assets/presign`, {
    environment_id: source.id, project_id: sourceProject.id, filename: 'asset.txt', content_type: 'text/plain', size: assetBody.length,
  })
  let upload
  if (ticket.upload_method === 'POST') {
    upload = await request.post(ticket.upload_url, {
      multipart: {
        ...ticket.form_fields,
        file: { name: 'asset.txt', mimeType: 'text/plain', buffer: assetBody },
      },
      headers: ticket.headers,
    })
  } else {
    upload = await request.fetch(ticket.upload_url, {
      method: ticket.upload_method,
      data: assetBody,
      headers: ticket.headers,
    })
  }
  expect(upload.ok(), await upload.text()).toBeTruthy()
  const completed = await api(request, token, 'POST', `/admin/environments/${source.id}/projects/${sourceProject.id}/assets/complete`, {
    environment_id: source.id, project_id: sourceProject.id, object_key: ticket.object_key,
  })
  expect(completed.asset.size).toBe(String(assetBody.length))

  const snapshotReply = await api(request, token, 'POST', `/admin/environments/${source.id}/snapshot/create`, {
    environment_id: source.id, project_id: sourceProject.id, config_id: configReply.config.id,
    description: 'E2E release', tags: ['RELEASE'],
  })
  const published = await api(request, token, 'POST', `/admin/environments/${source.id}/snapshots/${snapshotReply.snapshot.id}/publish`, {
    environment_id: source.id, snapshot_id: snapshotReply.snapshot.id, version: snapshotReply.snapshot.version,
  })
  expect(published.snapshot.status).toBe('RELEASED')

  const keyReply = await api(request, token, 'POST', `/admin/environments/${source.id}/projects/${sourceProject.id}/api-keys`, {
    environment_id: source.id, project_id: sourceProject.id, name: 'e2e',
  })
  expect(keyReply.secret).toMatch(/^kirby_pk_/)
  const runtime = await request.get('/api/v1/config', {
    params: { project: sourceProject.key, key: configReply.config.key },
    headers: { 'X-Kirby-API-Key': keyReply.secret },
  })
  expect(runtime.ok(), await runtime.text()).toBeTruthy()
  expect((await runtime.json()).content).toBe('"hello from Kirby"')

  const imported = await api(request, token, 'POST', `/admin/environments/${target.id}/snapshot-imports`, {
    target_environment_id: target.id, source_environment_id: source.id,
    source_snapshot_id: snapshotReply.snapshot.id, target_project_id: targetProject.id,
    description: 'Imported by E2E', tags: ['REUSE'], idempotency_key: 'kirby-e2e-import-0001', conflict_strategy: 'FAIL',
  })
  const replayed = await api(request, token, 'POST', `/admin/environments/${target.id}/snapshot-imports`, {
    target_environment_id: target.id, source_environment_id: source.id,
    source_snapshot_id: snapshotReply.snapshot.id, target_project_id: targetProject.id,
    description: 'Imported by E2E', tags: ['REUSE'], idempotency_key: 'kirby-e2e-import-0001', conflict_strategy: 'FAIL',
  })
  expect(imported.snapshot.id).toBe(replayed.snapshot.id)
  expect(replayed.replayed).toBe(true)

  const rotated = await api(request, token, 'POST', `/admin/environments/${source.id}/projects/${sourceProject.id}/api-keys/${keyReply.api_key.id}/rotate`, {
    environment_id: source.id, project_id: sourceProject.id, key_id: keyReply.api_key.id,
  })
  expect(rotated.secret).not.toBe(keyReply.secret)
  const oldKeyResponse = await request.get('/api/v1/config', {
    params: { project: sourceProject.key, key: configReply.config.key }, headers: { 'X-Kirby-API-Key': keyReply.secret },
  })
  expect(oldKeyResponse.status()).toBe(401)

  const listedKeys = await api(request, token, 'GET', `/admin/environments/${source.id}/projects/${sourceProject.id}/api-keys`)
  expect(JSON.stringify(listedKeys)).not.toContain(rotated.secret)

  for (const [roleId, roleName, expectedPermission] of [
    [1, 'viewer', 'project:read'], [2, 'editor', 'config:write'],
    [3, 'publisher', 'snapshot:publish'], [4, 'environment-admin', 'project:api_key:manage'],
  ]) {
    const userReply = await api(request, token, 'POST', '/admin/users', {
      username: `e2e-${roleName}`, display_name: roleName, password: roleCredential, is_system_admin: false,
    })
    await api(request, token, 'PUT', `/admin/environments/${source.id}/users/${userReply.user.id}/roles`, {
      environment_id: source.id, user_id: userReply.user.id, role_ids: [roleId],
    })
    const roleLogin = await loginByAPI(request, `e2e-${roleName}`, roleCredential)
    const permissions = await api(request, roleLogin.access_token, 'GET', `/admin/environments/${source.id}/my-permissions`)
    expect(permissions.permissions).toContain(expectedPermission)
    expect(permissions.permissions.includes('system:user:manage')).toBe(false)
  }

  let refreshCount = 0
  page.on('response', (response) => {
    if (response.url().endsWith('/api/auth/refresh')) refreshCount += 1
  })

  await page.getByRole('combobox', { name: '当前环境' }).press('ArrowDown')
  const targetOption = page.getByText('Target', { exact: true }).filter({ visible: true })
  await expect(targetOption).toBeVisible()

  let expiredTokenInjected = false
  await page.route('**/api/admin/**', async (route) => {
    if (expiredTokenInjected) {
      await route.continue()
      return
    }
    expiredTokenInjected = true
    await route.continue({
      headers: {
        ...route.request().headers(),
        authorization: `Bearer ${expiredAccessToken(loginReply.user.id)}`,
      },
    })
  })

  await targetOption.click()
  await expect(page.getByText('Target Project')).toBeVisible()
  await expect(page.getByText('Source Project')).not.toBeVisible()
  await page.unroute('**/api/admin/**')
  expect(expiredTokenInjected).toBe(true)
  expect(refreshCount).toBe(1)
})
