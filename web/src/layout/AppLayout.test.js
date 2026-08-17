// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AppLayout from './AppLayout.vue'

function mountLayout({
  systemAdmin = false,
  permissions = [],
  routeName = 'home',
  dispatch = vi.fn().mockResolvedValue(),
} = {}) {
  const replace = vi.fn().mockResolvedValue()
  return shallowMount(AppLayout, {
    mocks: {
      $message: { error: vi.fn() },
      $route: { name: routeName },
      $router: { replace },
      $store: {
        dispatch,
        getters: {
          'environment/current': null,
          'environment/hasPermission': (permission) =>
            permissions.includes(permission),
          'session/systemAdmin': systemAdmin,
        },
        state: {
          environment: {
            available: [],
            currentId: null,
            switching: false,
          },
          session: { user: { username: 'tester' } },
        },
      },
    },
    stubs: [
      'el-button',
      'el-option',
      'el-select',
      'EnvironmentTag',
      'router-link',
      'router-view',
    ],
  })
}

describe('AppLayout administration entrance', () => {
  it('is visible to system administrators', () => {
    expect(mountLayout({ systemAdmin: true }).vm.showAdministration).toBe(true)
  })

  it('is visible to current-environment administrators', () => {
    expect(
      mountLayout({
        permissions: ['environment:member:manage'],
      }).vm.showAdministration,
    ).toBe(true)
  })

  it('is hidden from ordinary members', () => {
    expect(
      mountLayout({ permissions: ['project:read'] }).vm.showAdministration,
    ).toBe(false)
  })

  it('shows the project entrance to ordinary members', () => {
    const wrapper = mountLayout({ permissions: ['project:read'] })
    const links = wrapper.findAllComponents({ name: 'RouterLink' })

    expect(
      links.wrappers.some(
        (link) => link.attributes('to') === '[object Object]',
      ),
    ).toBe(true)
    expect(wrapper.text()).toContain('项目')
  })
})

describe('AppLayout environment switching', () => {
  it('returns to the project list after a successful environment switch', async () => {
    const dispatch = vi.fn().mockResolvedValue()
    const wrapper = mountLayout({ routeName: 'config-detail', dispatch })

    await wrapper.vm.switchEnvironment(22)

    expect(dispatch).toHaveBeenCalledWith('environment/select', 22)
    expect(wrapper.vm.$router.replace).toHaveBeenCalledWith({
      name: 'project-list',
    })
  })

  it('does not navigate again when already on the project list', async () => {
    const wrapper = mountLayout({ routeName: 'project-list' })

    await wrapper.vm.switchEnvironment(22)

    expect(wrapper.vm.$router.replace).not.toHaveBeenCalled()
  })
})
