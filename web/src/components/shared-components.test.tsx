import { render, screen } from '@testing-library/react'
import { StrictMode } from 'react'
import { describe, expect, it, vi } from 'vitest'

const monaco = vi.hoisted(() => {
  const subscription = { dispose: vi.fn() }
  const model = {
    dispose: vi.fn(),
    setValue: vi.fn(),
  }
  const editorInstance = {
    dispose: vi.fn(),
    getModel: vi.fn(() => model),
    getValue: vi.fn(() => 'initial'),
    onDidChangeModelContent: vi.fn(() => subscription),
    setValue: vi.fn(),
    updateOptions: vi.fn(),
  }
  const diffInstance = {
    dispose: vi.fn(),
    setModel: vi.fn(),
  }
  return {
    subscription,
    model,
    editorInstance,
    diffInstance,
    create: vi.fn(() => editorInstance),
    createDiffEditor: vi.fn(() => diffInstance),
    createModel: vi.fn(() => ({ ...model, dispose: vi.fn(), setValue: vi.fn() })),
    setModelLanguage: vi.fn(),
  }
})

vi.mock('monaco-editor/editor/editor.api', () => ({
  editor: {
    create: monaco.create,
    createDiffEditor: monaco.createDiffEditor,
    createModel: monaco.createModel,
    setModelLanguage: monaco.setModelLanguage,
  },
}))
vi.mock('monaco-editor/language/json/monaco.contribution', () => ({}))
vi.mock('./MonacoEditor/monaco-environment', () => ({
  configureMonacoWorkers: vi.fn(),
}))

import DataTypeSelector from './DataTypeSelector/DataTypeSelector'
import { buildDataTypeGroups } from './DataTypeSelector/data-type-groups'
import DiffEditor from './DiffEditor/DiffEditor'
import EnvironmentTag from './EnvironmentTag/EnvironmentTag'
import { environmentTagStyle } from './EnvironmentTag/environment-tag-style'
import MonacoEditor from './MonacoEditor/MonacoEditor'

describe('shared editor lifecycle', () => {
  it('disposes Monaco instances and subscriptions in Strict Mode', () => {
    const rendered = render(
      <StrictMode>
        <MonacoEditor value="initial" />
        <DiffEditor leftValue={{ value: 1 }} rightValue={{ value: 2 }} />
      </StrictMode>,
    )

    rendered.unmount()

    expect(monaco.create).toHaveBeenCalledTimes(2)
    expect(monaco.createDiffEditor).toHaveBeenCalledTimes(2)
    expect(monaco.subscription.dispose).toHaveBeenCalledTimes(2)
    expect(monaco.editorInstance.dispose).toHaveBeenCalledTimes(2)
    expect(monaco.diffInstance.dispose).toHaveBeenCalledTimes(2)
    const createdModels = monaco.createModel.mock.results.map((result) => result.value)
    expect(createdModels).toHaveLength(4)
    createdModels.forEach((model) => expect(model.dispose).toHaveBeenCalled())
  })
})

describe('shared selectors and tags', () => {
  it('groups base, enum, and model types', () => {
    const groups = buildDataTypeGroups({
      models: [{ key: 'Server', name: '服务器' }],
      enums: [{ key: 'State', name: '状态' }],
      limitedModels: [],
      limit: false,
    })
    expect(groups.map((group) => group.label)).toEqual(['基本类型', '枚举', '模型'])
    expect(groups[1]?.options).toEqual([
      { label: '状态', value: '{"enumKey":"State"}' },
    ])
  })

  it('renders a selector and stable environment label', () => {
    render(
      <>
        <DataTypeSelector optionGroups={[{ label: '模型', options: [{ label: '服务器', value: 'server' }] }]} />
        <EnvironmentTag
          environment={{ id: 1, key: 'east', name: '华东环境', enabled: true, version: 1 }}
        />
      </>,
    )
    expect(screen.getByText('华东环境')).toBeInTheDocument()
    expect(environmentTagStyle({ id: 1, key: 'east', name: '华东环境', enabled: true, version: 1 })).not.toEqual(
      environmentTagStyle({ id: 2, key: 'west', name: '西部环境', enabled: true, version: 1 }),
    )
  })
})
