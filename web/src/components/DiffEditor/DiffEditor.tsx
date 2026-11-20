import { useEffect, useRef } from 'react'
import type * as Monaco from 'monaco-editor/editor/editor.api'

import { formatDiffValue } from '@/domain/format-diff'
import { useTranslation } from '@/i18n'
import { loadMonaco } from '@/components/MonacoEditor/monaco-runtime'

type Props = { leftValue: unknown; rightValue: unknown }

export default function DiffEditor({ leftValue, rightValue }: Props) {
  const container = useRef<HTMLDivElement | null>(null)
  const originalModel = useRef<Monaco.editor.ITextModel | null>(null)
  const modifiedModel = useRef<Monaco.editor.ITextModel | null>(null)
  const latestLeftValue = useRef(leftValue)
  const latestRightValue = useRef(rightValue)
  const { locale } = useTranslation()
  latestLeftValue.current = leftValue
  latestRightValue.current = rightValue

  useEffect(() => {
    let active = true
    if (!container.current) throw new Error('DiffEditor container is missing')
    let diffEditor: Monaco.editor.IStandaloneDiffEditor | null = null
    let original: Monaco.editor.ITextModel | null = null
    let modified: Monaco.editor.ITextModel | null = null
    void loadMonaco(locale).then((monaco) => {
      if (!active || !container.current) return
      diffEditor = monaco.editor.createDiffEditor(container.current, {
        automaticLayout: true,
        wordWrap: 'on',
        diffWordWrap: 'on',
        folding: true,
        theme: 'vs-light',
        readOnly: true,
        bracketPairColorization: { enabled: true },
      })
      original = monaco.editor.createModel(formatDiffValue(latestLeftValue.current), 'json')
      modified = monaco.editor.createModel(formatDiffValue(latestRightValue.current), 'json')
      originalModel.current = original
      modifiedModel.current = modified
      diffEditor.setModel({ original, modified })
    })
    return () => {
      active = false
      diffEditor?.setModel(null)
      original?.dispose()
      modified?.dispose()
      diffEditor?.dispose()
      originalModel.current = null
      modifiedModel.current = null
    }
  }, [locale])

  useEffect(() => {
    originalModel.current?.setValue(formatDiffValue(leftValue))
  }, [leftValue])

  useEffect(() => {
    modifiedModel.current?.setValue(formatDiffValue(rightValue))
  }, [rightValue])

  return <div ref={container} className="diff-editor" />
}
