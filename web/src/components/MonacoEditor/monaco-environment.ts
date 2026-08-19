import EditorWorker from 'monaco-editor/editor/editor.worker?worker'
import JsonWorker from 'monaco-editor/language/json/json.worker?worker'

let configured = false

type MonacoWorkerEnvironment = {
  getWorker: (_moduleId: string, label: string) => Worker
}

export function configureMonacoWorkers(): void {
  if (configured) return
  ;(globalThis as typeof globalThis & { MonacoEnvironment: MonacoWorkerEnvironment })
    .MonacoEnvironment = {
    getWorker: (_moduleId, label) =>
      label === 'json' ? new JsonWorker() : new EditorWorker(),
  }
  configured = true
}
