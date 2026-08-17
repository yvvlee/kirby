// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AppLayout from './AppLayout.vue'

function mountLayout({ systemAdmin = false, permissions = [] } = {}) {
  return shallowMount(AppLayout, {
    mocks: {
      $store: {
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
})
