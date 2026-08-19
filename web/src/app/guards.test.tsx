import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'

import { AuthContext, type AuthContextValue } from '@/auth/auth-state'
import {
  EnvironmentContext,
  type EnvironmentContextValue,
} from '@/auth/environment-state'
import { AuthGate, PermissionGate } from './guards'

const user = {
  id: 1,
  username: 'member',
  is_system_admin: false,
  version: 1,
}

function authValue(overrides: Partial<AuthContextValue> = {}): AuthContextValue {
  return {
    user,
    authenticated: true,
    systemAdmin: false,
    initialized: true,
    busy: false,
    error: null,
    login: vi.fn(),
    logout: vi.fn(),
    bootstrap: vi.fn(async () => true),
    expire: vi.fn(),
    ...overrides,
  }
}

function environmentValue(
  overrides: Partial<EnvironmentContextValue> = {},
): EnvironmentContextValue {
  return {
    available: [],
    current: null,
    currentId: null,
    permissions: [],
    switching: false,
    initialized: true,
    error: null,
    hasPermission: (permission) =>
      overrides.permissions?.includes(permission) ?? false,
    loadAvailable: vi.fn(async () => []),
    select: vi.fn(async () => null),
    resetScope: vi.fn(async () => undefined),
    ...overrides,
  }
}

function LoginLocation() {
  const location = useLocation()
  return <output aria-label="登录地址">{`${location.pathname}${location.search}`}</output>
}

describe('route guards', () => {
  it('redirects anonymous users with a local return path', () => {
    render(
      <AuthContext.Provider
        value={authValue({ user: null, authenticated: false })}
      >
        <MemoryRouter initialEntries={['/projects?keyword=demo']}>
          <Routes>
            <Route path="/login" element={<LoginLocation />} />
            <Route
              path="/projects"
              element={<AuthGate><span>项目</span></AuthGate>}
            />
          </Routes>
        </MemoryRouter>
      </AuthContext.Provider>,
    )

    expect(screen.getByLabelText('登录地址')).toHaveTextContent(
      '/login?redirect=%2Fprojects%3Fkeyword%3Ddemo',
    )
  })

  it('redirects a member without the required permission', () => {
    render(
      <AuthContext.Provider value={authValue()}>
        <EnvironmentContext.Provider value={environmentValue()}>
          <MemoryRouter initialEntries={['/projects']}>
            <Routes>
              <Route path="/403" element={<span>没有权限</span>} />
              <Route
                path="/projects"
                element={<PermissionGate permission="project:read"><span>项目</span></PermissionGate>}
              />
            </Routes>
          </MemoryRouter>
        </EnvironmentContext.Provider>
      </AuthContext.Provider>,
    )

    expect(screen.getByText('没有权限')).toBeInTheDocument()
  })

  it('allows a declared backend permission', () => {
    render(
      <AuthContext.Provider value={authValue()}>
        <EnvironmentContext.Provider
          value={environmentValue({ permissions: ['project:read'] })}
        >
          <MemoryRouter initialEntries={['/projects']}>
            <PermissionGate permission="project:read"><span>项目</span></PermissionGate>
          </MemoryRouter>
        </EnvironmentContext.Provider>
      </AuthContext.Provider>,
    )

    expect(screen.getByText('项目')).toBeInTheDocument()
  })
})
