import { act, render, waitFor } from '@testing-library/react'
import { createRef, StrictMode } from 'react'
import { describe, expect, it } from 'vitest'

import type { SchemaNode } from '@/domain/schema'
import SchemaForm, { type SchemaFormHandle } from './SchemaForm'

const config: SchemaNode = {
  value: {
    key: 'message',
    name: '消息',
    description: '',
    isArray: false,
    type: { baseType: 'String' },
  },
  children: [],
}

describe('SchemaForm', () => {
  it('normalizes serialized values and keeps the same form across disabled changes', async () => {
    const ref = createRef<SchemaFormHandle>()
    const { rerender } = render(
      <StrictMode>
        <SchemaForm ref={ref} config={config} value={'"old"'} />
      </StrictMode>,
    )

    await waitFor(() => expect(ref.current?.getValue()).toBe('old'))
    const initialForm = ref.current?.form
    act(() => ref.current?.setValue('"new"'))
    expect(ref.current?.getValue()).toBe('new')

    rerender(
      <StrictMode>
        <SchemaForm ref={ref} config={config} value={'"new"'} disabled />
      </StrictMode>,
    )

    await waitFor(() => expect(ref.current?.form.pattern).toBe('disabled'))
    expect(ref.current?.form).toBe(initialForm)
  })

  it('fails fast when a file field has no scoped upload component', () => {
    const fileConfig: SchemaNode = {
      ...config,
      value: { ...config.value, type: { baseType: 'File' } },
    }

    expect(() => render(<SchemaForm config={fileConfig} value={'""'} />)).toThrow(
      '文件字段需要通过 fileUploadComponent 显式注入上传组件',
    )
  })
})
