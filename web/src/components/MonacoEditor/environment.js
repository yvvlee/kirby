import editorWorker from 'monaco-editor/editor/editor.worker?worker'
import jsonWorker from 'monaco-editor/language/json/json.worker?worker'

let configured = false

export const configureMonacoWorkers = () => {
  if (configured) {
    return
  }
  globalThis.MonacoEnvironment = {
    getWorker(_, label) {
      if (label === 'json') {
        return new jsonWorker()
      }
      return new editorWorker()
    },
  }
  configured = true
}
