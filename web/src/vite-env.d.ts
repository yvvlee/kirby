/// <reference types="vite/client" />

declare module 'monaco-editor/editor/editor.api?kirbyLocale=zh-CN' {
  export * from 'monaco-editor/editor/editor.api'
}

declare module 'monaco-editor/editor/editor.api?kirbyLocale=en-US' {
  export * from 'monaco-editor/editor/editor.api'
}

declare module 'monaco-editor/editor/contrib/find/browser/findController?kirbyLocale=zh-CN' {}
declare module 'monaco-editor/editor/contrib/find/browser/findController?kirbyLocale=en-US' {}
declare module 'monaco-editor/editor/contrib/contextmenu/browser/contextmenu?kirbyLocale=zh-CN' {}
declare module 'monaco-editor/editor/contrib/contextmenu/browser/contextmenu?kirbyLocale=en-US' {}
declare module 'monaco-editor/editor/contrib/clipboard/browser/clipboard?kirbyLocale=zh-CN' {}
declare module 'monaco-editor/editor/contrib/clipboard/browser/clipboard?kirbyLocale=en-US' {}

declare module 'monaco-editor/language/json/monaco.contribution?kirbyLocale=zh-CN' {}
declare module 'monaco-editor/language/json/monaco.contribution?kirbyLocale=en-US' {}

declare module 'monaco-editor/editor/editor.worker?worker&kirbyLocale=zh-CN' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

declare module 'monaco-editor/editor/editor.worker?worker&kirbyLocale=en-US' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

declare module 'monaco-editor/language/json/json.worker?worker&kirbyLocale=zh-CN' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}

declare module 'monaco-editor/language/json/json.worker?worker&kirbyLocale=en-US' {
  const WorkerFactory: new () => Worker
  export default WorkerFactory
}
