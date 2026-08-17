import { describe, expect, it, vi } from 'vitest'

import FileUpload from './FileUpload.vue'
import { createScopedFileUpload } from './scoped'

describe('scoped file upload adapter', () => {
  it('injects the explicit page scope and preserves Formily field data', () => {
    const component = createScopedFileUpload(11, 7)
    const createElement = vi.fn((tag, data, children) => ({
      children,
      data,
      tag,
    }))
    const onChange = vi.fn()
    const rendered = component.render(createElement, {
      children: [],
      data: {
        attrs: { field: { value: '', onChange } },
        props: {
          environmentId: 99,
          isArray: true,
          projectId: 99,
          uploadType: 'File',
        },
      },
      listeners: { input: vi.fn() },
      props: {},
    })

    expect(rendered.tag).toBe(FileUpload)
    expect(rendered.data.props).toMatchObject({
      environmentId: 11,
      projectId: 7,
      uploadType: 'File',
      isArray: true,
    })
    expect(rendered.data.attrs.field.onChange).toBe(onChange)
  })

  it('fails before rendering an invalid page scope', () => {
    expect(() => createScopedFileUpload(0, 7)).toThrow(
      'environmentId must be a positive integer',
    )
    expect(() => createScopedFileUpload(11, null)).toThrow(
      'projectId must be a positive integer',
    )
  })
})
