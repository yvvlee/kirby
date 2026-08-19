import { useEffect, useRef } from 'react'
import * as monaco from 'monaco-editor/editor/editor.api'
import 'monaco-editor/language/json/monaco.contribution'

import { configureMonacoWorkers } from '@/components/MonacoEditor/monaco-environment'
import { formatDiffValue } from '@/domain/format-diff'

configureMonacoWorkers()

type Props = { leftValue: unknown; rightValue: unknown }

export default function DiffEditor({ leftValue, rightValue }: Props) {
  const container = useRef<HTMLDivElement | null>(null)
  const originalModel = useRef<monaco.editor.ITextModel | null>(null)
  const modifiedModel = useRef<monaco.editor.ITextModel | null>(null)
  const initialLeftValue = useRef(leftValue)
  const initialRightValue = useRef(rightValue)

  useEffect(() => {
    if (!container.current) throw new Error('DiffEditor container is missing')
    const diffEditor = monaco.editor.createDiffEditor(container.current, {
      automaticLayout: true,
      wordWrap: 'on',
      diffWordWrap: 'on',
      folding: true,
      theme: 'vs-light',
      readOnly: true,
      bracketPairColorization: { enabled: true },
    })
    const original = monaco.editor.createModel(formatDiffValue(initialLeftValue.current), 'json')
    const modified = monaco.editor.createModel(formatDiffValue(initialRightValue.current), 'json')
    originalModel.current = original
    modifiedModel.current = modified
    diffEditor.setModel({ original, modified })
    return () => {
      diffEditor.setModel(null)
      original.dispose()
      modified.dispose()
      diffEditor.dispose()
      originalModel.current = null
      modifiedModel.current = null
    }
  }, [])

  useEffect(() => {
    originalModel.current?.setValue(formatDiffValue(leftValue))
  }, [leftValue])

  useEffect(() => {
    modifiedModel.current?.setValue(formatDiffValue(rightValue))
  }, [rightValue])

  return <div ref={container} className="diff-editor" />
}
