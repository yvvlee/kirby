import { useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor/editor/editor.api'
import 'monaco-editor/language/json/monaco.contribution'

import { configureMonacoWorkers } from './monaco-environment'

configureMonacoWorkers()

type Props = {
  value?: string
  disabled?: boolean
  language?: string
  onChange?: (value: string) => void
}

export default function MonacoEditor({
  value = '',
  disabled = false,
  language = 'json',
  onChange,
}: Props) {
  const container = useRef<HTMLDivElement | null>(null)
  const editor = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  const onChangeRef = useRef(onChange)
  const initialValue = useRef(value)
  const initialLanguage = useRef(language)
  const initialDisabled = useRef(disabled)

  useEffect(() => {
    onChangeRef.current = onChange
  }, [onChange])

  useEffect(() => {
    if (!container.current) throw new Error('MonacoEditor container is missing')
    const instance = monaco.editor.create(container.current, {
      value: initialValue.current,
      language: initialLanguage.current,
      automaticLayout: true,
      minimap: { enabled: false },
      readOnly: initialDisabled.current,
    })
    editor.current = instance
    const subscription = instance.onDidChangeModelContent(() => {
      onChangeRef.current?.(instance.getValue())
    })
    return () => {
      subscription.dispose()
      instance.dispose()
      editor.current = null
    }
  }, [])

  useEffect(() => {
    const instance = editor.current
    if (instance && instance.getValue() !== value) instance.setValue(value)
  }, [value])

  useEffect(() => {
    editor.current?.updateOptions({ readOnly: disabled })
  }, [disabled])

  useEffect(() => {
    const model = editor.current?.getModel()
    if (model) monaco.editor.setModelLanguage(model, language)
  }, [language])

  return <div ref={container} className="monaco-editor-container" />
}
