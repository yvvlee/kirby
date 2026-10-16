import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const requests = vi.hoisted(() => ({
  getSetupStatus: vi.fn(),
  createInitialAdministrator: vi.fn(),
}))
vi.mock('@/api/setup', () => requests)

import { AuthContext, type AuthContextValue } from '@/auth/auth-state'
import LoginPage from '@/routes/LoginPage'
import SetupPage from '@/routes/SetupPage'
import { SetupGate } from './setup-gate'

function renderInstallation(path: string, authenticated = false) {
  const auth: AuthContextValue = {
    user: null,
    authenticated,
    systemAdmin: authenticated,
    initialized: true,
    busy: false,
    error: null,
    login: vi.fn(),
    logout: vi.fn(),
    bootstrap: vi.fn(),
    expire: vi.fn(),
  }
  return render(
    <AuthContext.Provider value={auth}>
      <MemoryRouter initialEntries={[path]}>
        <SetupGate>
          <Routes>
            <Route path="/setup" element={<SetupPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/" element={<h1>配置管理平台</h1>} />
            <Route path="/projects/:projectId/configs" element={<h1>项目配置</h1>} />
          </Routes>
        </SetupGate>
      </MemoryRouter>
    </AuthContext.Provider>,
  )
}

async function fillAdministrator(password = 'initial-password-123', confirmation = password) {
  const user = userEvent.setup()
  await screen.findByRole('heading', { name: '创建初始管理员' })
  await user.type(screen.getByRole('textbox', { name: '用户名' }), 'owner')
  await user.type(screen.getByRole('textbox', { name: '显示名' }), '管理员')
  await user.click(screen.getByLabelText('密码', { exact: true }))
  await user.paste(password)
  await user.click(screen.getByLabelText('确认密码'))
  await user.paste(confirmation)
  await user.click(screen.getByRole('button', { name: '创建管理员' }))
}

beforeEach(() => {
  vi.resetAllMocks()
  requests.getSetupStatus.mockResolvedValue({ initialized: false })
  requests.createInitialAdministrator.mockResolvedValue(undefined)
})

describe('installation routing', () => {
  it.each(['/login', '/projects/12/configs'])('routes an empty installation from %s to setup', async (path) => {
    renderInstallation(path)
    expect(await screen.findByRole('heading', { name: '创建初始管理员' })).toBeVisible()
    expect(screen.queryByRole('heading', { name: '登录配置管理平台' })).not.toBeInTheDocument()
    expect(requests.getSetupStatus).toHaveBeenCalledTimes(1)
  })

  it('preserves a deep link once the installation has been initialized', async () => {
    requests.getSetupStatus.mockResolvedValue({ initialized: true })
    renderInstallation('/projects/12/configs')
    expect(await screen.findByRole('heading', { name: '项目配置' })).toBeVisible()
  })

  it('keeps status failures visible and retries without guessing the installation state', async () => {
    requests.getSetupStatus.mockRejectedValueOnce(new Error('服务暂不可用'))
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: true })
    renderInstallation('/login')

    expect(await screen.findByText('无法读取初始化状态')).toBeVisible()
    expect(screen.queryByRole('heading', { name: '创建初始管理员' })).not.toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '登录配置管理平台' })).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /重\s*试/ }))
    expect(await screen.findByRole('heading', { name: '登录配置管理平台' })).toBeVisible()
  })

  it.each([false, true])('leaves setup after initialization, authenticated=%s', async (authenticated) => {
    requests.getSetupStatus.mockResolvedValue({ initialized: true })
    renderInstallation('/setup', authenticated)
    const heading = authenticated ? '配置管理平台' : '登录配置管理平台'
    expect(await screen.findByRole('heading', { name: heading })).toBeVisible()
    expect(requests.createInitialAdministrator).not.toHaveBeenCalled()
  })
})

describe('initial administrator form', () => {
  it('creates an administrator, refreshes installation state and offers login', async () => {
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: false })
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: true })
    renderInstallation('/setup')
    await fillAdministrator()

    expect(await screen.findByText('管理员已创建，请登录')).toBeVisible()
    expect(screen.getByRole('heading', { name: '登录配置管理平台' })).toBeVisible()
    expect(requests.createInitialAdministrator).toHaveBeenCalledWith({
      username: 'owner',
      display_name: '管理员',
      password: 'initial-password-123',
    })
    expect(requests.getSetupStatus).toHaveBeenCalledTimes(2)
  })

  it('rejects mismatching passwords before making a request', async () => {
    renderInstallation('/setup')
    await fillAdministrator('initial-password-123', 'another-password-123')
    expect(await screen.findByText('两次输入的密码不一致')).toBeVisible()
    expect(requests.createInitialAdministrator).not.toHaveBeenCalled()
  })

  it.each(['short', '密'.repeat(342)])('rejects passwords outside the UTF-8 byte limits', async (password) => {
    renderInstallation('/setup')
    await fillAdministrator(password)
    expect(await screen.findByText('密码长度须为 12 至 1024 字节')).toBeVisible()
    expect(requests.createInitialAdministrator).not.toHaveBeenCalled()
  })

  it('accepts a multibyte password containing twelve UTF-8 bytes', async () => {
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: false })
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: true })
    renderInstallation('/setup')
    await fillAdministrator('管理密码')
    expect(await screen.findByText('管理员已创建，请登录')).toBeVisible()
    expect(requests.createInitialAdministrator).toHaveBeenCalledWith({
      username: 'owner',
      display_name: '管理员',
      password: '管理密码',
    })
  })

  it('shows account creation failures without redirecting', async () => {
    requests.createInitialAdministrator.mockRejectedValue(new Error('创建失败'))
    renderInstallation('/setup')
    await fillAdministrator()
    expect(await screen.findByText('创建失败')).toBeVisible()
    expect(screen.getByRole('heading', { name: '创建初始管理员' })).toBeVisible()
  })

  it('refreshes state after another visitor completes setup', async () => {
    requests.createInitialAdministrator.mockRejectedValue({
      isAxiosError: true,
      response: { status: 409 },
    })
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: false })
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: true })
    renderInstallation('/setup')
    await fillAdministrator()

    expect(await screen.findByRole('heading', { name: '登录配置管理平台' })).toBeVisible()
    expect(screen.queryByText('管理员已创建，请登录')).not.toBeInTheDocument()
    expect(requests.getSetupStatus).toHaveBeenCalledTimes(2)
  })

  it('shows a retryable error if the post-conflict status check fails', async () => {
    requests.createInitialAdministrator.mockRejectedValue({
      isAxiosError: true,
      response: { status: 409 },
    })
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: false })
    requests.getSetupStatus.mockRejectedValueOnce(new Error('连接已断开'))
    requests.getSetupStatus.mockResolvedValueOnce({ initialized: true })
    renderInstallation('/setup')
    await fillAdministrator()

    expect(await screen.findByText('无法读取初始化状态')).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: /重\s*试/ }))
    expect(await screen.findByRole('heading', { name: '登录配置管理平台' })).toBeVisible()
  })
})
