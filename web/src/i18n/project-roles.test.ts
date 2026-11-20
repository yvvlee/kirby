import { afterEach, describe, expect, it } from 'vitest'
import { setLocale } from './index'
import { projectRoleLabel } from './project-roles'

afterEach(() => {
  setLocale('zh-CN')
})

describe('projectRoleLabel', () => {
  it('localizes fixed role codes without changing their values', () => {
    expect(projectRoleLabel('OWNER')).toBe('所有者')
    setLocale('en-US')
    expect(projectRoleLabel('OWNER')).toBe('Owner')
  })

  it('preserves unknown role codes', () => {
    expect(projectRoleLabel('CUSTOM_ROLE')).toBe('CUSTOM_ROLE')
  })
})
