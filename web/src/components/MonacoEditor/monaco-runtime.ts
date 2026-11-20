import type * as Monaco from 'monaco-editor/editor/editor.api'

import { configureMonacoWorkers } from './monaco-environment'

type Locale = 'zh-CN' | 'en-US'
type WorkerFactoryModule = { default: new () => Worker }

let loading = Promise.resolve()

/**
 * Loads Monaco's editor, language contribution, and workers from one locale-specific module graph.
 * Parameters: locale selects Chinese NLS messages or Monaco's English source messages.
 * Returns: the Monaco editor API after the matching worker environment is configured.
 * Throws: module loading errors when Monaco assets cannot be loaded.
 */
export function loadMonaco(locale: Locale): Promise<typeof Monaco> {
  const current = loading.then(() => loadLocale(locale))
  loading = current.then(() => undefined, () => undefined)
  return current
}

async function loadLocale(locale: Locale): Promise<typeof Monaco> {
  if (locale === 'zh-CN') {
    const [api, , , , , editorWorker, jsonWorker] = await Promise.all([
      import('monaco-editor/editor/editor.api?kirbyLocale=zh-CN'),
      import('monaco-editor/editor/contrib/find/browser/findController?kirbyLocale=zh-CN'),
      import('monaco-editor/editor/contrib/contextmenu/browser/contextmenu?kirbyLocale=zh-CN'),
      import('monaco-editor/editor/contrib/clipboard/browser/clipboard?kirbyLocale=zh-CN'),
      import('monaco-editor/language/json/monaco.contribution?kirbyLocale=zh-CN'),
      import('monaco-editor/editor/editor.worker?worker&kirbyLocale=zh-CN') as Promise<WorkerFactoryModule>,
      import('monaco-editor/language/json/json.worker?worker&kirbyLocale=zh-CN') as Promise<WorkerFactoryModule>,
    ])
    configureMonacoWorkers(editorWorker.default, jsonWorker.default)
    return api
  }
  const [api, , , , , editorWorker, jsonWorker] = await Promise.all([
    import('monaco-editor/editor/editor.api?kirbyLocale=en-US'),
    import('monaco-editor/editor/contrib/find/browser/findController?kirbyLocale=en-US'),
    import('monaco-editor/editor/contrib/contextmenu/browser/contextmenu?kirbyLocale=en-US'),
    import('monaco-editor/editor/contrib/clipboard/browser/clipboard?kirbyLocale=en-US'),
    import('monaco-editor/language/json/monaco.contribution?kirbyLocale=en-US'),
    import('monaco-editor/editor/editor.worker?worker&kirbyLocale=en-US') as Promise<WorkerFactoryModule>,
    import('monaco-editor/language/json/json.worker?worker&kirbyLocale=en-US') as Promise<WorkerFactoryModule>,
  ])
  configureMonacoWorkers(editorWorker.default, jsonWorker.default)
  return api
}
