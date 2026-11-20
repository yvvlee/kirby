import { useEffect, useRef } from 'react'
import type * as Monaco from 'monaco-editor/editor/editor.api'

import { useTranslation } from '@/i18n'
import { loadMonaco } from './monaco-runtime'

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
  const editor = useRef<Monaco.editor.IStandaloneCodeEditor | null>(null)
  const onChangeRef = useRef(onChange)
  const currentValue = useRef(value)
  const disabledRef = useRef(disabled)
  const { locale } = useTranslation()
  disabledRef.current = disabled

  useEffect(() => {
    onChangeRef.current = onChange
  }, [onChange])

  useEffect(() => {
    let active = true
    let disposeSubscription: (() => void) | null = null
    if (!container.current) throw new Error('MonacoEditor container is missing')
    void loadMonaco(locale).then((monaco) => {
      if (!active || !container.current) return
      const instance = monaco.editor.create(container.current, {
        value: currentValue.current,
        language,
        automaticLayout: true,
        minimap: { enabled: false },
        readOnly: disabledRef.current,
      })
      editor.current = instance
      const subscription = instance.onDidChangeModelContent(() => {
        const nextValue = instance.getValue()
        currentValue.current = nextValue
        onChangeRef.current?.(nextValue)
      })
      disposeSubscription = () => subscription.dispose()
    })
    return () => {
      active = false
      disposeSubscription?.()
      editor.current?.dispose()
      editor.current = null
    }
  }, [language, locale])

  useEffect(() => {
    const instance = editor.current
    currentValue.current = value
    if (instance && instance.getValue() !== value) instance.setValue(value)
  }, [value])

  useEffect(() => {
    editor.current?.updateOptions({ readOnly: disabled })
  }, [disabled])

  return <div ref={container} className="monaco-editor-container" />
}
