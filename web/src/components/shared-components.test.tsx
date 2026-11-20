import { render, screen, waitFor } from '@testing-library/react'
import { StrictMode } from 'react'
import { describe, expect, it, vi } from 'vitest'

const monaco = vi.hoisted(() => {
  const subscription = { dispose: vi.fn() }
  const contentListener = vi.fn()
  const model = {
    dispose: vi.fn(),
    setValue: vi.fn(),
  }
  const editorInstance = {
    dispose: vi.fn(),
    getModel: vi.fn(() => model),
    getValue: vi.fn(() => 'initial'),
    onDidChangeModelContent: vi.fn((listener?: () => void) => {
      if (listener) contentListener.mockImplementation(listener)
      return subscription
    }),
    setValue: vi.fn(),
    updateOptions: vi.fn(),
  }
  const diffInstance = {
    dispose: vi.fn(),
    setModel: vi.fn(),
  }
  const create = vi.fn<
    (container: HTMLElement, options: unknown) => typeof editorInstance
  >(() => editorInstance)
  const createDiffEditor = vi.fn(() => diffInstance)
  const createModel = vi.fn(() => ({ ...model, dispose: vi.fn(), setValue: vi.fn() }))
  return {
    subscription,
    model,
    editorInstance,
    diffInstance,
    create,
    createDiffEditor,
    createModel,
    setModelLanguage: vi.fn(),
    api: {
      editor: {
        create,
        createDiffEditor,
        createModel,
      },
    },
    contentListener,
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
vi.mock('./MonacoEditor/monaco-runtime', () => ({
  loadMonaco: vi.fn(() => Promise.resolve(monaco.api)),
}))
vi.mock('./MonacoEditor/monaco-environment', () => ({
  configureMonacoWorkers: vi.fn(),
}))

import DataTypeSelector from './DataTypeSelector/DataTypeSelector'
import { buildDataTypeGroups } from './DataTypeSelector/data-type-groups'
import DiffEditor from './DiffEditor/DiffEditor'
import EnvironmentTag from './EnvironmentTag/EnvironmentTag'
import { environmentTagStyle } from './EnvironmentTag/environment-tag-style'
import MonacoEditor from './MonacoEditor/MonacoEditor'
import LanguageSwitcher from '@/i18n/LanguageSwitcher'
import { setLocale } from '@/i18n'
import { loadMonaco } from './MonacoEditor/monaco-runtime'

describe('shared editor lifecycle', () => {
  it('disposes Monaco instances and subscriptions in Strict Mode', async () => {
    const rendered = render(
      <StrictMode>
        <MonacoEditor value="initial" />
        <DiffEditor leftValue={{ value: 1 }} rightValue={{ value: 2 }} />
      </StrictMode>,
    )

    await waitFor(() => expect(monaco.create).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(monaco.createDiffEditor).toHaveBeenCalledTimes(1))

    rendered.unmount()

    expect(monaco.create).toHaveBeenCalledTimes(1)
    expect(monaco.createDiffEditor).toHaveBeenCalledTimes(1)
    expect(monaco.subscription.dispose).toHaveBeenCalledTimes(1)
    expect(monaco.editorInstance.dispose).toHaveBeenCalledTimes(1)
    expect(monaco.diffInstance.dispose).toHaveBeenCalledTimes(1)
    const createdModels = monaco.createModel.mock.results.map((result) => result.value)
    expect(createdModels).toHaveLength(2)
    createdModels.forEach((model) => expect(model.dispose).toHaveBeenCalled())
  })

  it('recreates Monaco in the selected locale without losing current input', async () => {
    setLocale('zh-CN')
    const rendered = render(
      <>
        <LanguageSwitcher />
        <MonacoEditor value="initial" onChange={() => undefined} />
      </>,
    )
    await waitFor(() => expect(monaco.create).toHaveBeenCalled())

    monaco.editorInstance.getValue.mockReturnValue('edited text')
    monaco.contentListener()
    setLocale('en-US')

    await waitFor(() => expect(monaco.create.mock.calls.at(-1)?.[1]).toMatchObject({
      value: 'edited text',
    }))
    rendered.unmount()
    setLocale('zh-CN')
  })

  it('ignores stale locale loads after a rapid switch and keeps edited text', async () => {
    setLocale('zh-CN')
    let resolveInitial: (value: unknown) => void = () => undefined
    const initialLoad = new Promise<unknown>((resolve) => {
      resolveInitial = resolve
    })
    vi.mocked(loadMonaco).mockImplementationOnce(
      () => initialLoad as ReturnType<typeof loadMonaco>,
    )

    const rendered = render(<MonacoEditor value="initial" onChange={() => undefined} />)
    setLocale('en-US')
    await waitFor(() => expect(monaco.create).toHaveBeenCalled())
    monaco.editorInstance.getValue.mockReturnValue('edited before locale settled')
    monaco.contentListener()
    setLocale('zh-CN')
    await waitFor(() => expect(monaco.create.mock.calls.at(-1)?.[1]).toMatchObject({
      value: 'edited before locale settled',
    }))

    const callsBeforeStaleLoad = monaco.create.mock.calls.length
    resolveInitial(monaco.api)
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(monaco.create).toHaveBeenCalledTimes(callsBeforeStaleLoad)
    rendered.unmount()
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
