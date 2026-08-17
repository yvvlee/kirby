// @vitest-environment jsdom

import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import SystemLayout from './SystemLayout.vue'

function mountLayout({ systemAdmin = false, permissions = [] } = {}) {
  return shallowMount(SystemLayout, {
    mocks: {
      $store: {
        getters: { 'session/systemAdmin': systemAdmin },
        state: { environment: { permissions } },
      },
    },
    stubs: ['router-link', 'router-view'],
  })
}

describe('SystemLayout navigation', () => {
  it('shows all management links to a system administrator', () => {
    const wrapper = mountLayout({ systemAdmin: true })
    expect(wrapper.vm.visibleLinks.map((link) => link.name)).toEqual([
      'system-home',
      'system-environments',
      'system-users',
      'system-roles',
      'environment-members',
    ])
  })

  it('shows only environment membership to an environment administrator', () => {
    const wrapper = mountLayout({
      permissions: ['environment:member:manage'],
    })
    expect(wrapper.vm.visibleLinks.map((link) => link.name)).toEqual([
      'system-home',
      'environment-members',
    ])
  })

  it('shows no write entrance to ordinary members', () => {
    const wrapper = mountLayout({ permissions: ['project:read'] })
    expect(wrapper.vm.visibleLinks.map((link) => link.name)).toEqual([
      'system-home',
    ])
  })
})
