// @vitest-environment jsdom

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import EnvironmentTag from './EnvironmentTag.vue'

describe('EnvironmentTag', () => {
  it('使用接口返回的名称并根据环境键生成颜色', () => {
    const first = mount(EnvironmentTag, {
      propsData: {
        environment: { key: 'east', name: '华东环境' },
      },
    })
    const second = mount(EnvironmentTag, {
      propsData: {
        environment: { key: 'west', name: '西部环境' },
      },
    })

    expect(first.text()).toBe('华东环境')
    expect(first.vm.tagStyle).not.toEqual(second.vm.tagStyle)
  })
})
