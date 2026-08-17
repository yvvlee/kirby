// @vitest-environment jsdom

import { describe, expect, it, vi } from 'vitest'

vi.mock('monaco-editor/editor/editor.api', () => ({ editor: {} }))
vi.mock('monaco-editor/language/json/monaco.contribution', () => ({}))
vi.mock('@formily/core', () => ({ createForm: vi.fn() }))
vi.mock('@formily/vue', () => ({
  FormProvider: { name: 'FormProvider' },
  createSchemaField: () => ({ SchemaField: { name: 'SchemaField' } }),
}))
vi.mock('./formily-components.js', () => {
  const component = (name) => ({ name })
  return {
    ArrayCards: component('ArrayCards'),
    ArrayItems: component('ArrayItems'),
    DatePicker: component('DatePicker'),
    FormItem: component('FormItem'),
    Input: component('FormInput'),
    InputNumber: component('InputNumber'),
    Select: component('FormSelect'),
    Space: component('FormSpace'),
    Switch: component('FormSwitch'),
    TimePicker: component('TimePicker'),
  }
})

import {
  DataTypeSelector,
  DiffEditor,
  MonacoEditor,
  SchemaForm,
} from '@/features/config-center/schema/index.js'

describe('编辑器组件入口', () => {
  it('可以在 Vite 中解析全部组件', () => {
    expect(SchemaForm.name).toBe('SchemaForm')
    expect(MonacoEditor.name).toBe('MonacoEditor')
    expect(DiffEditor.name).toBe('DiffEditor')
    expect(DataTypeSelector.name).toBe('DataTypeSelector')
  })
})
